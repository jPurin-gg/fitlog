"""Check go test -json output for real integration coverage, not skips."""

import json
import sys

# One integration-only sentinel per suite; all tests still run via go test.
REQUIRED_TESTS = {
    "github.com/jPurin-gg/myfitlog-backend/internal/app": "TestManualWorkoutWithoutMonthlyPlanHTTP",
    "github.com/jPurin-gg/myfitlog-backend/internal/workout/postgres": "TestRecordSetConcurrentReplay",
}


def check(events):
    passed_tests = set()
    passed_packages = set()
    for event in events:
        action = event.get("Action")
        package = event.get("Package")
        test = event.get("Test")
        if action in {"skip", "fail"}:
            raise ValueError(f"Integration test {action}: {package} {test or ''}")
        if action == "pass":
            if test:
                passed_tests.add((package, test))
            else:
                passed_packages.add(package)
    for package, test in REQUIRED_TESTS.items():
        if package not in passed_packages or (package, test) not in passed_tests:
            raise ValueError(f"Missing successful integration suite: {package} ({test})")
    print(f"Verified integration suites; {len(passed_tests)} tests/subtests passed, no skips")


if __name__ == "__main__":
    with open(sys.argv[1], encoding="utf-8") as stream:
        check(json.loads(line) for line in stream)
