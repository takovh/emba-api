import os
from pathlib import Path

EMBA_PATH = os.environ.get("EMBA_PATH", "/home/gst/yzhang/emba")
EMBA_LOG_BASE_DIR = os.environ.get("EMBA_LOG_BASE_DIR", "/home/gst/yzhang/emba-log")
EMBA_MAX_CONCURRENT_SCANS = int(os.environ.get("EMBA_MAX_CONCURRENT_SCANS", "1"))
EMBA_HOST = os.environ.get("EMBA_HOST", "0.0.0.0")
EMBA_PORT = int(os.environ.get("EMBA_PORT", "8203"))

EMBA_BIN = str(Path(EMBA_PATH) / "emba")
EMBA_VERSION_FILE = str(Path(EMBA_PATH) / "config" / "VERSION.txt")
EMBA_SCAN_PROFILES_DIR = str(Path(EMBA_PATH) / "scan-profiles")

os.makedirs(EMBA_LOG_BASE_DIR, exist_ok=True)
