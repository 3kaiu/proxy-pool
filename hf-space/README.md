---
title: Proxy Pool Relay
emoji: 🔄
colorFrom: blue
colorTo: green
sdk: gradio
app_file: app.py
pinned: false
---

# Proxy Pool Relay

Lightweight Go relay that proxies requests to opencode.ai/zen through free public proxies.

## Endpoints

- `GET /v1/models` — list available models
- `POST /v1/chat/completions` — chat completions (OpenAI-compatible)
- `GET /count` — proxy pool status
