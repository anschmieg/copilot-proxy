# OpenAI API Compatibility Plan

This plan outlines all steps required to make the application fully OpenAI API compatible, including endpoint structure, request/response formats, error handling, and streaming.

---

## 1. Endpoints

### 1.1. `/v1/models` (GET)
- **Status:** Implemented and compliant (with filtering).
- **Action:** Optionally add `/v1/models` as an alias to `/models` for strict compatibility.

### 1.2. `/v1/chat/completions` (POST)
- **Status:** Implemented at `/openai` and `/v1/chat/completions`.
- **Action:** Ensure non-streaming responses include all required OpenAI fields:
  - `id`, `object`, `created`, `model`, `choices`, `usage`
- **Streaming:** Ensure each SSE chunk matches OpenAI's format (`data: {...}\n\n`, ends with `data: [DONE]\n\n`).

### 1.3. `/v1/completions` (POST) (optional)
- **Status:** Not implemented.
- **Action:** Implement if you want to support legacy GPT-3 completions. Must accept OpenAI's completion request and respond with OpenAI's completion response format.

### 1.4. `/v1/embeddings` (POST) (optional)
- **Status:** Not implemented.
- **Action:** Implement if you want to support OpenAI embeddings API. Must accept OpenAI's embeddings request and respond with OpenAI's embeddings response format.

---

## 2. Request/Response Format

### 2.1. Chat Completion Response (non-streaming)
- **Required fields:**
  - `id`: string (unique for each response)
  - `object`: "chat.completion"
  - `created`: unix timestamp
  - `model`: model id used
  - `choices`: array of completions
  - `usage`: token usage stats

- **Action:**  
  - In `HandleCompletion`, after collecting the full response, wrap it in the above structure.
  - Generate a unique `id` (e.g., "chatcmpl-<random>").
  - Set `created` to `time.Now().Unix()`.
  - Set `model` to the model used.
  - Calculate or estimate `usage` if possible.

- **Non-compliance:**  
  - If any of these fields are missing in the response, the API is not fully OpenAI-compatible.

### 2.2. Streaming Response
- **Required:**  
  - Each chunk must be a JSON object prefixed with `data: ` and terminated with `\n\n`.
  - End with `data: [DONE]\n\n`.

- **Action:**  
  - Ensure each streamed chunk matches OpenAI's streaming format.

### 2.3. Error Responses
- **Required format:**
  ```json
  {
    "error": {
      "message": "Error message",
      "type": "error_type",
      "param": null,
      "code": null
    }
  }
  ```
- **Action:**  
  - Replace all uses of `http.Error` and similar with a helper that writes this JSON format and sets the correct HTTP status code.
  - Use appropriate error types (`invalid_request_error`, `api_error`, etc.).

- **Non-compliance:**  
  - If any endpoint returns plain text or HTML errors, or does not use this structure, it is not OpenAI-compliant.

---

## 3. Headers

- **Required:**  
  - `Content-Type: application/json` for JSON responses.
  - `Content-Type: text/event-stream` for streaming.
  - `Authorization: Bearer <token>` for all endpoints.

- **Action:**  
  - Already handled, but double-check for all endpoints.

---

## 4. Authentication

- **Required:**  
  - Accept `Authorization: Bearer <token>` for all endpoints.
  - Support for disabling auth via env/config for development.

- **Action:**  
  - Already implemented.

---

## 5. Optional Endpoints

- `/v1/files`, `/v1/fine-tunes`, `/v1/audio/transcriptions`, `/v1/images/generations`, etc.
- **Action:**  
  - Implement only if you want to support OpenAI fine-tuning, file uploads, Whisper, or DALL·E.

---

## 6. Testing

- **Action:**  
  - Use OpenAI client libraries (e.g., openai-python, openai-node) to test all endpoints.
  - Validate with curl and compare responses to OpenAI's API.
  - Test both streaming and non-streaming modes.

---

## 7. Documentation

- **Action:**  
  - Document all supported endpoints and their compatibility in README or a dedicated doc file.
  - Clearly state any limitations or deviations from the OpenAI API.

---

## 8. Summary Table

| Endpoint               | Status      | Action Needed/Non-compliance                 |
| ---------------------- | ----------- | -------------------------------------------- |
| `/v1/models`           | ✅ Compliant | Optionally add alias for `/v1/models`        |
| `/v1/chat/completions` | ✅ Compliant |                                              |
| `/v1/completions`      | ❌ Missing   | Implement if legacy support is desired       |
| `/v1/embeddings`       | ❌ Missing   | Implement if embedding support is desired    |
| Error responses        | ✅ Compliant |                                              |
| Streaming              | ✅*          | Compliant if Copilot upstream is OpenAI-like |

---

## 9. Known Non-Compliant Elements

- **Legacy endpoints** (`/v1/completions`, `/v1/embeddings`) are not implemented.  
  - **Fix:** Implement if you want full OpenAI API coverage.
- **Streaming:**  
  - If Copilot upstream changes its streaming format, you may need to adapt your proxy logic.
- **Token usage stats:**  
  - If Copilot upstream does not provide usage stats, the `usage` field may be missing or inaccurate in non-streaming responses.
- **Model filtering:**  
  - `/v1/models` filters by country and user, which is stricter than OpenAI's default. Not a compliance issue, but worth documenting.

---

**Conclusion:**  
Your app is fully OpenAI-compatible for chat completions and model listing.  
It does not implement legacy completions or embeddings endpoints, and token usage stats may be limited by Copilot's upstream.  
No critical compliance issues for chat-based OpenAI clients.
