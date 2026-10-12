"""Fail closed unless all required jobs completed successfully."""

import json
import os

REQUIRED_JOBS = {"frontend", "backend", "db-integration", "e2e"}


def check(results):
    if set(results) != REQUIRED_JOBS:
        raise ValueError("Required job results are missing or unexpected")
    for job, result in results.items():
        print(f"{job}: {result['result']}")
        if result["result"] != "success":
            raise ValueError(f"{job} did not succeed")


if __name__ == "__main__":
    check(json.loads(os.environ["NEEDS_JSON"]))
