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

1. Each entry is a dash list item in this exact format:
   - **Title** ([#PR](url)) - Closes [#N](url)
     - sub-bullet 1
     - sub-bullet 2
2. Keep ALL markdown structure exactly as-is: headings (##, ###), dash list items (- **...**), and blank lines between entries. Do NOT add horizontal rules (---) between entries.
3. Keep ALL PR links ([#NNN](url)) and issue close references (Closes [#N](url), Fixes [#N](url)) exactly as-is. Do not modify URLs or numbers.
4. Rewrite the bold title to be a clear, concise phrase a non-developer can understand. Do not include conventional commit prefixes.
5. Rewrite sub-bullets to focus on what changed for the user, not implementation details. Avoid referencing internal function names, database columns, or code structure. Keep 2 spaces of indentation before each sub-bullet.
6. Keep 2-3 sub-bullets per entry. Combine related points if there are more. Keep single bullets if there is only one.
7. Do not invent new information. Only rephrase what is already there.
8. Do not add emojis.
9. Use backticks for config keys, file paths, CLI flags, and command names.
10. If a title looks like garbage (e.g., "New (3):", "### Files", random fragments), replace it with a clear summary derived from the sub-bullets.

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
