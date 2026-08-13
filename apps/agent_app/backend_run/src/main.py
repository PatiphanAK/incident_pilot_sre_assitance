from fastapi import FastAPI
from datetime import datetime, timezone
import time

app = FastAPI()


@app.get("/health/live", tags=["Health"])
def liveness():
    return {"status": "UP"}
