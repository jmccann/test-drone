# test-drone-deployment

This is a golang app to test an internal drone deployment.

Currently running via `drone exec`.  Once the tests have stabilized will probably
publish a binary you can just call directly (and not have to wait for compile).

```
drone exec --secret DRONE_SERVER=$DRONE_SERVER --secret DRONE_TOKEN=$DRONE_TOKEN --secret GITHUB_TOKEN=$GITHUB_TOKEN --secret GITHUB_BASEURL=$GITHUB_BASEURL
```

* `DRONE_SERVER` - The drone server to test.
* `DRONE_TOKEN` - Your DRONE_TOKEN to the DRONE_SERVER to be able to hit the APIs
* `GITHUB_TOKEN` - A personal GHE token so app can generate a commit to trigger a new drone build

# Dependencies

## General

This is dependent on having a repo setup like `junk_repo` in this project.  That
repo drives integration testing of secrets and stress testing of log generation.

## Integration

The following secret/values are expected to have been set:

* `NOT_CONCEALED`: `MYSUPERSECRETsecret`
* `IS_CONCEALED`: can be anything

## Stress

This was used against a server with 2 agents (4cpux8gb) running 3 concurrent jobs.
If you have more resources (more agents, more concurrent builds, etc) avail
stress jobs may finish too quickly to catch log stream read issues.
