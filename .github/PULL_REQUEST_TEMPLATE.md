## Summary

- 

## Verification

- [ ] `scripts/ci-local.sh`
- [ ] `scripts/release-smoke.sh`
- [ ] `git diff --check`
- [ ] `scripts/public-safety-check.sh`
- [ ] Parent workspace private-marker grep returned no matches when publishing
- [ ] Temporary release/live artifacts cleanup checks returned no matches

## Hosted CI

- [ ] Hosted GitHub Actions ran real workflow steps
- [ ] If Hosted CI did not start because of billing or spending limits, link the
      failing run and keep `scripts/ci-local.sh` as the verification source

## Production Backlog

- [ ] No new production gate introduced
- [ ] New or changed production gates are tracked in `docs/PRODUCTION_BACKLOG.md`
      and linked GitHub issues

## public/private boundary

- [ ] This PR does not include tenant endpoints, account names, mailbox
      addresses, passwords, OAuth tokens, cookies, canary values, private policy
      links, raw mailbox content, or raw session artifacts
