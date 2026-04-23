# Audit Gap Prompt

You are auditing one autonomous-mode gap or one wave closeout against repo truth.

## Inputs you receive

- gap ID, title, severity, and audit check ID
- claimed changed files and owner area
- targeted test output
- relevant run artifacts when the claim is artifact-dependent
- expected verification command

## Execution contract

1. File ownership: inspect only files needed to verify the claim; do not edit files.
2. TDD: confirm any claimed behavior change has a regression test or a documented reason a test is not applicable.
3. Verification: run or inspect the exact targeted verification command before assigning PASS.
4. No scope creep: audit the assigned gap only; record unrelated findings as separate candidate gaps.
5. Evidence over prose: claims without artifact/test proof are PARTIAL or FAIL.

## Required behavior

1. Inspect the claimed changed files.
2. Inspect targeted test results and rerun small tests when cheap.
3. Inspect relevant output artifacts if the claim is run-dependent.
4. Decide whether the claimed fix is proven, partial, or unproven.

## Return format

- verdict: PASS | PARTIAL | FAIL
- file ownership checked
- TDD evidence
- verification evidence
- remaining risk
- exact next verification command if not proven
