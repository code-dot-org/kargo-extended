# Local Smoke Notes

- Primary smoke entrypoint:
  - `extended/plugins/send-message-go/smoke/smoke-test.sh`
- The script assumes a working Kargo cluster and CLI are already available.
- Required env:
  - `SEND_MESSAGE_SMOKE_PROJECT`
  - `SEND_MESSAGE_SMOKE_WAREHOUSE`
  - `SEND_MESSAGE_SMOKE_FREIGHT_NAME`
  - `SEND_MESSAGE_SMOKE_SLACK_API_KEY`
  - `SEND_MESSAGE_SMOKE_CHANNEL_ID`
- Optional env:
  - `KARGO_BIN`
  - `KARGO_FLAGS`
  - `SEND_MESSAGE_SMOKE_SYSTEM_RESOURCES_NAMESPACE`
- No committed file in this subtree contains a real token value or local token
  source.
