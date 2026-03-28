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
  --endpoint https://openrouter.ai/api/v1/chat/completions \
  --key <YOUR_OPENROUTER_API_KEY> \
  --model openai/gpt-4o-mini
```

Then start a chat session:

```bash
noto chat
```
