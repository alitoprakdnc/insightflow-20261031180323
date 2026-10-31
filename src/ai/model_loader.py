import os
import threading
import logging
from pathlib import Path
from typing import Any, Iterable, List, Union

import joblib
import numpy as np
from sklearn.base import BaseEstimator

logger = logging.getLogger(__name__)

# Environment variable that points to the serialized model file.
# Defaults to a file named `model.pkl` located in the same directory as this module.
DEFAULT_MODEL_PATH = Path(__file__).with_name("model.pkl")
MODEL_PATH_ENV = "ANOMALY_MODEL_PATH"


class ModelLoadError(RuntimeError):
    """Raised when the anomaly detection model cannot be loaded."""


class ModelPredictionError(RuntimeError):
    """Raised when the model fails to produce a prediction."""


class ModelWrapper:
    """
    Thread‑safe wrapper around a scikit‑learn model.

    The wrapper lazily loads the model on first use and protects the
    ``predict`` call with a re‑entrant lock, making it safe to be called
    from multiple FastAPI request workers concurrently.
    """

    _instance_lock = threading.Lock()
    _model_instance: Union[BaseEstimator, None] = None
    _load_lock = threading.RLock()

    def __init__(self, model_path: Path) -> None:
        self._model_path = model_path
        self._ensure_model_loaded()

    def _ensure_model_loaded(self) -> None:
        """
        Load the model from disk if it hasn't been loaded yet.
        This method is protected by a re‑entrant lock to avoid race conditions
        when multiple threads attempt to load the model simultaneously.
        """
        if self.__class__._model_instance is not None:
            return

        with self.__class__._load_lock:
            if self.__class__._model_instance is not None:
                # Another thread loaded the model while we were waiting for the lock.
                return

            if not self._model_path.is_file():
                raise ModelLoadError(
                    f"Model file not found at '{self._model_path}'. "
                    f"Set the environment variable {MODEL_PATH_ENV} to a valid path."
                )

            try:
                logger.info("Loading anomaly detection model from %s", self._model_path)
                model = joblib.load(self._model_path)
                if not isinstance(model, BaseEstimator):
                    raise ModelLoadError(
                        f"The loaded object from '{self._model_path}' is not a scikit‑learn estimator."
                    )
                self.__class__._model_instance = model
                logger.info("Model loaded successfully.")
            except Exception as exc:
                logger.exception("Failed to load model from %s", self._model_path)
                raise ModelLoadError(f"Unable to load model: {exc}") from exc

    @property
    def model(self) -> BaseEstimator:
        """Return the underlying scikit‑learn model (guaranteed to be loaded)."""
        # The model is loaded lazily; ensure it exists before returning.
        self._ensure_model_loaded()
        assert self.__class__._model_instance is not None  # for type‑checkers
        return self.__class__._model_instance

    def predict(
        self,
        data: Union[Iterable[Iterable[float]], np.ndarray, List[List[float]]],
    ) -> List[Any]:
        """
        Perform a thread‑safe prediction.

        Parameters
        ----------
        data: array‑like
            2‑D array where each row represents a feature vector for a KPI sample.

        Returns
        -------
        List[Any]
            The model's raw predictions (e.g., anomaly scores or class labels).

        Raises
        ------
        ModelPredictionError
            If the underlying model raises an exception during prediction.
        """
        # Convert input to a NumPy array with shape (n_samples, n_features)
        try:
            X = np.asarray(data, dtype=float)
            if X.ndim != 2:
                raise ValueError(
                    f"Input data must be 2‑dimensional (got shape {X.shape})."
                )
        except Exception as exc:
            logger.exception("Invalid input data for prediction")
            raise ModelPredictionError(f"Invalid input data: {exc}") from exc

        # Protect the actual prediction call.
        with self.__class__._load_lock:
            try:
                logger.debug("Running model prediction on %d samples", X.shape[0])
                raw_pred = self.model.predict(X)
                # Ensure the result is a plain Python list for JSON serialisation.
                return raw_pred.tolist() if hasattr(raw_pred, "tolist") else list(raw_pred)
            except Exception as exc:
                logger.exception("Model prediction failed")
                raise ModelPredictionError(f"Prediction error: {exc}") from exc


# --------------------------------------------------------------------------- #
# Module‑level singleton handling
# --------------------------------------------------------------------------- #

_global_wrapper: Union[ModelWrapper, None] = None
_global_lock = threading.Lock()


def _get_global_wrapper() -> ModelWrapper:
    """
    Return a singleton ``ModelWrapper`` instance, creating it on first call.
    This function is thread‑safe and guarantees that only one model is ever
    loaded per Python process.
    """
    global _global_wrapper
    if _global_wrapper is not None:
        return _global_wrapper

    with _global_lock:
        if _global_wrapper is None:
            model_path_str = os.getenv(MODEL_PATH_ENV, str(DEFAULT_MODEL_PATH))
            model_path = Path(model_path_str).expanduser().resolve()
            _global_wrapper = ModelWrapper(model_path)
        return _global_wrapper


def predict(
    data: Union[Iterable[Iterable[float]], np.ndarray, List[List[float]]],
) -> List[Any]:
    """
    Convenience function used by the FastAPI endpoint.

    It forwards the call to the singleton ``ModelWrapper`` instance,
    providing a clean, import‑friendly API.

    Parameters
    ----------
    data: array‑like
        Same format as :meth:`ModelWrapper.predict`.

    Returns
    -------
    List[Any]
        Model predictions.
    """
    wrapper = _get_global_wrapper()
    return wrapper.predict(data)


__all__ = ["predict", "ModelLoadError", "ModelPredictionError", "ModelWrapper"]