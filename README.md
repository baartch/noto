# Noto

Noto is a local-first terminal chatbot with profile-scoped memory and a consistent TUI experience.

## Getting started

### 1) Create a profile

```bash
noto profile create "My Profile"
noto profile select "My Profile"
```

### 2) Configure a provider

```bash
noto provider set \
  --endpoint https://openrouter.ai/api/v1 \
  --key <YOUR_OPENROUTER_API_KEY> \
  --model openai/gpt-4o-mini
```

### 3) Select an embeddings model

In Settings (ctrl+j), pick **Model Embeddings** to enable memory retrieval.

Then start a chat session:

```bash
noto
```
