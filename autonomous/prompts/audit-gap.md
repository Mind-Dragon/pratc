# Audit Gap Prompt

You are auditing one gap or one wave closeout against repo truth.

Required behavior:
1. inspect the claimed changed files
2. inspect the targeted test results
3. inspect the relevant output artifacts if the claim is run-dependent
4. decide whether the claimed fix is proven, partial, or unproven

Return:
- verdict
- evidence
- remaining risk
- exact next verification command if not proven
