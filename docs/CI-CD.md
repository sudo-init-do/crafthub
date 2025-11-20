CI/CD for main and staging

- CI runs build and tests on every push and PR to `main` and `staging`.
- CD builds and publishes Docker images to `ghcr.io/sudo-init-do/crafthub` tagged by branch (e.g., `main`, `staging`) and by commit SHA.
- Environments: pushes to `main` are marked `production`; pushes to `staging` are marked `staging`.

Repository requirements
- Ensure `GITHUB_TOKEN` has `packages: write` permission (default in most repos).
- Optionally configure environment protection rules for `production` and `staging`.

Optional SSH deploy (gated by secrets)
- Add secrets: `DEPLOY_SSH_HOST`, `DEPLOY_SSH_USER`, `DEPLOY_SSH_KEY`, `DEPLOY_SSH_PORT`.
- When present, the workflow will `docker pull ghcr.io/sudo-init-do/crafthub:<branch>` and run `docker compose` remotely.
