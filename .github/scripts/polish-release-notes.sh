#!/bin/bash
# Polish release notes using OpenAI
# Usage: ./polish-release-notes.sh < raw-notes.md > polished-notes.md
#
# Takes structured release notes from generate-release-notes.sh and sends them
# to OpenAI (gpt-4o-mini) for polishing into clear, user-facing language.
#
# Falls back to raw notes if OPENAI_API_KEY is not set or API call fails.
#
# Requires: OPENAI_API_KEY environment variable, curl, jq

set -e

RAW_NOTES=$(cat)

# Graceful fallback if no API key
if [ -z "$OPENAI_API_KEY" ]; then
    echo "Warning: OPENAI_API_KEY not set, returning raw notes" >&2
    echo "$RAW_NOTES"
    exit 0
fi

# Build the system prompt
read -r -d '' PROMPT << 'PROMPT_EOF' || true
You are a technical writer for Scrutiny, an open-source hard drive health monitoring dashboard.

Rewrite the following release notes to be clear, concise, and user-facing. Follow these rules strictly:

1. Keep ALL markdown structure exactly as-is: headings (##, ###), bold titles (**...**), horizontal rules (---), and blank lines.
2. Keep ALL PR links ([#NNN](url)) and issue references (Closes #NNN, Fixes #NNN) exactly as-is. Do not modify URLs or numbers.
3. Rewrite the description line (the sentence after the bold title) to be a clear one-sentence summary that a non-developer user can understand.
4. Rewrite bullet points to focus on what changed for the user, not implementation details. Avoid referencing internal function names, database columns, or code structure.
5. Keep exactly 2-3 bullets per entry. Remove extras by combining related points. If there is only 1 bullet, keep it.
6. Do not invent new information. Only rephrase what is already there.
7. Do not add emojis.
8. Use backticks for config keys, file paths, CLI flags, and command names.
9. If a description line looks like garbage (e.g., "New (3):", "### Files", random fragments), replace it with a clear summary derived from the bullet points.

Return ONLY the polished markdown. No preamble, no explanation, no code fences wrapping the output.
PROMPT_EOF

# Escape for JSON
ESCAPED_NOTES=$(printf '%s' "$RAW_NOTES" | jq -Rs .)
ESCAPED_PROMPT=$(printf '%s' "$PROMPT" | jq -Rs .)

# Call OpenAI API
RESPONSE=$(curl -s --max-time 30 https://api.openai.com/v1/chat/completions \
    -H "Authorization: Bearer $OPENAI_API_KEY" \
    -H "Content-Type: application/json" \
    -d "{
        \"model\": \"gpt-4o-mini\",
        \"temperature\": 0.3,
        \"messages\": [
            {\"role\": \"system\", \"content\": $ESCAPED_PROMPT},
            {\"role\": \"user\", \"content\": $ESCAPED_NOTES}
        ]
    }" 2>/dev/null)

# Check for API errors
if [ $? -ne 0 ]; then
    echo "Warning: OpenAI API call failed, returning raw notes" >&2
    echo "$RAW_NOTES"
    exit 0
fi

# Extract the polished notes
POLISHED=$(echo "$RESPONSE" | jq -r '.choices[0].message.content // empty' 2>/dev/null)

if [ -z "$POLISHED" ]; then
    # Check if there was an API error message
    ERROR=$(echo "$RESPONSE" | jq -r '.error.message // empty' 2>/dev/null)
    if [ -n "$ERROR" ]; then
        echo "Warning: OpenAI API error: $ERROR" >&2
    else
        echo "Warning: OpenAI returned empty response, returning raw notes" >&2
    fi
    echo "$RAW_NOTES"
else
    echo "$POLISHED"
fi
