import React, { Component, ReactNode, ErrorInfo } from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Dashboard from './components/Dashboard';
import './index.css';

/**
 * Global error boundary that catches rendering errors.
 * In production it logs the error and shows a friendly UI.
 */
class ErrorBoundary extends Component<
  { children: ReactNode },
  { hasError: boolean; error?: Error }
> {
  constructor(props: { children: ReactNode }) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    // TODO: integrate with a monitoring service (e.g., Sentry, Datadog)
    console.error('Uncaught error in React tree:', error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div
          role="alert"
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            minHeight: '100vh',
            padding: '2rem',
            textAlign: 'center',
            backgroundColor: '#f8f9fa',
          }}
        >
          <h1>Something went wrong.</h1>
          <p>
            An unexpected error occurred. Please refresh the page or contact support if the
            problem persists.
          </p>
        </div>
      );
    }

    return this.props.children;
  }
}

/* -------------------------------------------------------------------------- */
/* Application bootstrap                                                       */
/* -------------------------------------------------------------------------- */

const rootElement = document.getElementById('root');
if (!rootElement) {
  throw new Error('Root element not found. Ensure an element with id="root" exists in index.html');
}

const root = ReactDOM.createRoot(rootElement);

root.render(
  <React.StrictMode>
    <ErrorBoundary>
      <BrowserRouter>
        <Routes>
          {/* Main dashboard – the entry point for the SaaS UI */}
          <Route path="/" element={<Dashboard />} />
          {/* Fallback: any unknown route redirects to the dashboard */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </ErrorBoundary>
  </React.StrictMode>
);