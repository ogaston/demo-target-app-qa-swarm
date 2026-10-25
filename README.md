# demo-target-app-qa-swarm

Reference target app for [Agentic QA Swarm](https://github.com/ogaston/agentic-qa-swarm): a small orders API
(`/products`, `/orders`) with a Postgres store.

**Invariant:** stock is never negative.

**Seeded defect:** start with `TARGET_APP_BUG=oversell` and `POST /orders` can sell more than the stock.
Leave it unset for the healthy app.

**Platform contract:** `GET /openapi.json`, a `baseline/baseline.sh` (`clean|verify|version`) and an
admin listener on `:9090` (token from a mounted file). See `docs/operaciones/app-objetivo.md` in the platform repo.

**Image:** every push to `main` publishes `ghcr.io/ogaston/demo-target-app-qa-swarm:<commit sha>`.
Make the package public so the cluster can pull it without credentials.
