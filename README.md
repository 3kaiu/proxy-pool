---
title: Proxy Pool Relay
emoji: 🔄
colorFrom: blue
colorTo: green
sdk: docker
app_port: 7860
pinned: false
---

# Proxy Pool Relay

Free proxy pool relay for opencode.ai zen endpoint.

## Usage

- **Models:** `GET /v1/models`
- **Chat:** `POST /v1/chat/completions`
- **Status:** `GET /count`

## Example

```bash
curl https://YOUR_SPACE_URL/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4-flash-free","messages":[{"role":"user","content":"hi"}],"max_tokens":100}'
```
