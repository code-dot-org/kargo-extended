# send-message-ts smoke

Run from `extended/plugins/send-message-ts/`:

```sh
SEND_MESSAGE_SMOKE_PROJECT=... \
SEND_MESSAGE_SMOKE_WAREHOUSE=... \
SEND_MESSAGE_SMOKE_FREIGHT_NAME=... \
SEND_MESSAGE_SMOKE_SLACK_API_KEY=... \
SEND_MESSAGE_SMOKE_CHANNEL_ID=... \
npm run smoke
```

Optional knobs:

- `SEND_MESSAGE_SMOKE_SYSTEM_RESOURCES_NAMESPACE`
- `SEND_MESSAGE_SMOKE_SECRET_NAME`
- `SEND_MESSAGE_SMOKE_CHANNEL_NAME`
- `KARGO_BIN`
- `KUBECTL_BIN`
- `DOCKER_BIN`
- `KIND_BIN`

This smoke path owns:

- image build
- kind image load
- plugin build dir render
- CRD and RBAC install
- local-only Secret and `MessageChannel`
- `Stage` creation and promotion polling

It assumes Kargo is already installed in the target kind cluster.

