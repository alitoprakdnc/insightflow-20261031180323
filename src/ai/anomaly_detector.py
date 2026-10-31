import os
import logging
from typing import List, Optional, Dict, Any

from fastapi import FastAPI, HTTPException, status, Request
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field, validator

# Local imports
from .model_loader import load_model, predict  # type: ignore

# --------------------------------------------------------------------------- #
# Configuration
# --------------------------------------------------------------------------- #

LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO").upper()
logging.basicConfig(
    level=LOG_LEVEL,
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger("anomaly_detector")

ANOMALY_THRESHOLD = float(os.getenv("ANOMALY_THRESHOLD", "0.5"))

# --------------------------------------------------------------------------- #
# Pydantic models
# --------------------------------------------------------------------------- #


class KPIRecord(BaseModel):
    """
    Represents a single KPI measurement.
    """

    timestamp: int = Field(
        ...,
        description="Unix epoch time in seconds (or milliseconds). "
        "The service does not enforce a specific unit – it is passed "
        "through to the model unchanged.",
    )
    metric: str = Field(..., description="Name of the KPI metric, e.g. 'revenue'.")
    value: float = Field(..., description="Measured value for the metric.")
    tags: Optional[Dict[str, Any]] = Field(
        default=None,
        description="Optional free‑form key/value pairs for dimensions "
        "(e.g. {'region': 'us-east', 'device': 'mobile'}).",
    )

    @validator("timestamp")
    def _positive_timestamp(cls, v: int) -> int:
        if v < 0:
            raise ValueError("timestamp must be a non‑negative integer")
        return v


class DetectRequest(BaseModel):
    """
    Payload sent to the /detect endpoint.
    """

    data: List[KPIRecord] = Field(..., min_items=1, description="Batch of KPI records.")


class DetectResponse(BaseModel):
    """
    Response returned by the /detect endpoint.
    """

    scores: List[float] = Field(..., description="Anomaly score for each input record.")
    anomalies: List[bool] = Field(
        ...,
        description=(
            "Boolean flag indicating whether the corresponding score exceeds the "
            "configured anomaly threshold."
        ),
    )


# --------------------------------------------------------------------------- #
# FastAPI application
# --------------------------------------------------------------------------- #

app = FastAPI(
    title="InsightFlow Anomaly Detector",
    version="1.0.0",
    description=(
        "FastAPI service that receives batches of KPI measurements, runs the "
        "pre‑trained anomaly‑detection model and returns per‑record scores."
    ),
)


# --------------------------------------------------------------------------- #
# Application lifecycle events
# --------------------------------------------------------------------------- #

_model = None  # type: ignore


@app.on_event("startup")
async def _load_ml_model() -> None:
    """
    Load the ML model once when the service starts.
    """
    global _model
    try:
        logger.info("Loading anomaly‑detection model...")
        _model = load_model()
        logger.info("Model loaded successfully.")
    except Exception as exc:
        logger.exception("Failed to load the anomaly‑detection model.")
        # Raising here prevents the app from starting, which is desirable.
        raise exc


@app.on_event("shutdown")
async def _shutdown() -> None:
    """
    Clean‑up resources if needed.
    """
    global _model
    _model = None
    logger.info("Shutting down anomaly‑detector service.")


# --------------------------------------------------------------------------- #
# Middleware (simple request/response logging)
# --------------------------------------------------------------------------- #


@app.middleware("http")
async def _log_requests(request: Request, call_next):
    logger.debug("Incoming request: %s %s", request.method, request.url.path)
    response = await call_next(request)
    logger.debug(
        "Response status: %s for %s %s", response.status_code, request.method, request.url.path
    )
    return response


# --------------------------------------------------------------------------- #
# Endpoints
# --------------------------------------------------------------------------- #


@app.get("/health", tags=["Health"])
async def health_check() -> Dict[str, str]:
    """
    Simple health endpoint used by orchestration tools.
    """
    return {"status": "ok"}


@app.post(
    "/detect",
    response_model=DetectResponse,
    status_code=status.HTTP_200_OK,
    tags=["Detection"],
    summary="Detect anomalies in a batch of KPI records",
)
async def detect_anomalies(payload: DetectRequest) -> DetectResponse:
    """
    Accepts a batch of KPI measurements, runs the pre‑trained model and returns
    an anomaly score for each record together with a boolean flag indicating
    whether the score exceeds the configured threshold.
    """
    if _model is None:
        logger.error("Model not loaded – this should never happen.")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Model not available.",
        )

    # Convert Pydantic objects to plain dicts for the model layer.
    records = [record.dict() for record in payload.data]

    try:
        raw_scores = predict(_model, records)  # Expected to return List[float] or np.ndarray
        # Normalise to a Python list of floats.
        if hasattr(raw_scores, "tolist"):
            scores = raw_scores.tolist()
        else:
            scores = list(raw_scores)  # type: ignore[arg-type]

        # Ensure we have a score per input record.
        if len(scores) != len(records):
            raise ValueError(
                f"Model returned {len(scores)} scores for {len(records)} input records."
            )

        anomalies = [float(s) > ANOMALY_THRESHOLD for s in scores]

        return DetectResponse(scores=scores, anomalies=anomalies)

    except HTTPException:
        # Propagate HTTP errors unchanged.
        raise
    except Exception as exc:
        logger.exception("Unexpected error during anomaly detection.")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to compute anomaly scores.",
        ) from exc


# --------------------------------------------------------------------------- #
# Run the service (useful for local development)
# --------------------------------------------------------------------------- #

if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "src.ai.anomaly_detector:app",
        host=os.getenv("HOST", "0.0.0.0"),
        port=int(os.getenv("PORT", "8000")),
        log_level=LOG_LEVEL.lower(),
        reload=os.getenv("DEBUG", "false").lower() == "true",
    )