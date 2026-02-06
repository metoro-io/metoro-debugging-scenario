import json
from datetime import datetime
from typing import Optional, Dict, Any


class StructuredLogger:
    def __init__(self, service_name: str):
        self.service_name = service_name

    def _log(self, level: str, message: str, fields: Optional[Dict[str, Any]] = None):
        """Write a structured log entry to stdout."""
        log_entry = {
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "level": level,
            "service_name": self.service_name,
            "message": message
        }

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
