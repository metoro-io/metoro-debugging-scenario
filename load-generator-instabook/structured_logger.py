import json
import sys
from datetime import datetime
from typing import Optional, Dict, Any
from opentelemetry import trace


class StructuredLogger:
    def __init__(self, service_name: str):
        self.service_name = service_name

    def _extract_trace_info(self) -> tuple[Optional[str], Optional[str]]:
        """Extract trace and span IDs from the current OpenTelemetry context."""
        span = trace.get_current_span()
        if span and span.get_span_context().is_valid:
            span_context = span.get_span_context()
            trace_id = format(span_context.trace_id, '032x')
            span_id = format(span_context.span_id, '016x')
            return trace_id, span_id
        return None, None

    def _log(self, level: str, message: str, fields: Optional[Dict[str, Any]] = None):
        """Write a structured log entry to stdout."""
        trace_id, span_id = self._extract_trace_info()

        log_entry = {
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "level": level,
            "service_name": self.service_name,
            "message": message
        }

        if trace_id:
            log_entry["trace_id"] = trace_id
        if span_id:
            log_entry["span_id"] = span_id
        if fields:
            log_entry["fields"] = fields

        print(json.dumps(log_entry), flush=True)

    def debug(self, message: str, **kwargs):
        """Log a debug message."""
        self._log("DEBUG", message, kwargs if kwargs else None)

    def info(self, message: str, **kwargs):
        """Log an info message."""
        self._log("INFO", message, kwargs if kwargs else None)

    def warning(self, message: str, **kwargs):
        """Log a warning message."""
        self._log("WARN", message, kwargs if kwargs else None)

    def error(self, message: str, **kwargs):
        """Log an error message."""
        self._log("ERROR", message, kwargs if kwargs else None)
