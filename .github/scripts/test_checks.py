import contextlib
import io
import unittest

import check_integration_tests
import check_required


class RequiredCheckTests(unittest.TestCase):
    def results(self):
        return {job: {"result": "success"} for job in check_required.REQUIRED_JOBS}

    def test_all_jobs_must_succeed(self):
        with contextlib.redirect_stdout(io.StringIO()):
            check_required.check(self.results())
            for job in check_required.REQUIRED_JOBS:
                for result in ("failure", "cancelled", "skipped"):
                    with self.subTest(job=job, result=result):
                        results = self.results()
                        results[job]["result"] = result
                        with self.assertRaises(ValueError):
                            check_required.check(results)

    def test_missing_job_is_rejected(self):
        for job in check_required.REQUIRED_JOBS:
            results = self.results()
            del results[job]
            with self.assertRaises(ValueError):
                check_required.check(results)


class IntegrationCheckTests(unittest.TestCase):
    def events(self):
        return [
            event
            for package, test in check_integration_tests.REQUIRED_TESTS.items()
            for event in (
                {"Action": "pass", "Package": package, "Test": test},
                {"Action": "pass", "Package": package},
            )
        ]

    def test_success_and_additional_tests(self):
        events = self.events() + [{"Action": "pass", "Package": "extra", "Test": "TestNew"}]
        with contextlib.redirect_stdout(io.StringIO()):
            check_integration_tests.check(events)

    def test_skip_or_failure_is_rejected_even_with_passing_sentinels(self):
        for action in ("skip", "fail"):
            with self.subTest(action=action), self.assertRaises(ValueError):
                check_integration_tests.check(self.events() + [{"Action": action, "Test": "TestOther"}])

    def test_empty_or_incomplete_output_is_rejected(self):
        with self.assertRaises(ValueError):
            check_integration_tests.check([])
        for index in range(len(self.events())):
            events = self.events()
            del events[index]
            with self.subTest(missing=index), self.assertRaises(ValueError):
                check_integration_tests.check(events)


if __name__ == "__main__":
    unittest.main()
