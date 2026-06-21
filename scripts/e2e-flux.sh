#!/bin/bash
# e2e-flux.sh — End-to-end test of FLUX.1 image generation pipeline
#
# Tests three layers:
#   1. flux binary can generate an image directly
#   2. task-runner API accepts and completes an image_generation task
#   3. thumbnails are generated for completed tasks
#
# Prerequisites:
#   - flux binary installed (~/.local/bin/flux)
#   - HF_TOKEN set (for model weight download on first run)
#   - task-runner service running on localhost:26706
#
# Usage:
#   ./scripts/e2e-flux.sh              # run all tests
#   ./scripts/e2e-flux.sh --binary     # test flux binary only
#   ./scripts/e2e-flux.sh --api        # test task-runner API only

set -euo pipefail

FLUX_BIN="${FLUX_BIN:-$HOME/.local/bin/flux}"
TASK_RUNNER_URL="${TASK_RUNNER_URL:-http://localhost:26706}"
OUTPUT_DIR=$(mktemp -d)
PASSED=0
FAILED=0
RUN_BINARY=true
RUN_API=true

while [[ $# -gt 0 ]]; do
    case "$1" in
        --binary) RUN_API=false; shift ;;
        --api) RUN_BINARY=false; shift ;;
        *) echo "Usage: $0 [--binary|--api]"; exit 1 ;;
    esac
done

cleanup() {
    rm -rf "$OUTPUT_DIR"
}
trap cleanup EXIT

pass() {
    local name=$1
    echo "  PASS  $name"
    PASSED=$((PASSED + 1))
}

fail() {
    local name=$1
    local reason=$2
    echo "  FAIL  $name — $reason"
    FAILED=$((FAILED + 1))
}

echo "=== FLUX.1 End-to-End Test ==="
echo "  flux binary: $FLUX_BIN"
echo "  task-runner: $TASK_RUNNER_URL"
echo "  output dir:  $OUTPUT_DIR"
echo ""

# ---------- Phase 1: Binary tests ----------

if [[ "$RUN_BINARY" == "true" ]]; then
    echo "--- Phase 1: flux binary ---"
    echo ""

    # Test 1.1: Binary exists and is executable
    if [[ -x "$FLUX_BIN" ]]; then
        pass "flux binary exists and is executable"
    else
        fail "flux binary exists" "$FLUX_BIN not found or not executable"
    fi

    # Test 1.2: Binary responds to --help
    if "$FLUX_BIN" --help &>/dev/null; then
        pass "flux --help succeeds"
    else
        fail "flux --help" "non-zero exit code"
    fi

    # Test 1.3: Generate an image
    IMAGE_PATH="$OUTPUT_DIR/test-binary.png"
    echo "  Generating image (this may take a while on first run)..."
    if "$FLUX_BIN" \
        --prompt "a red circle on a white background" \
        --output "$IMAGE_PATH" \
        --quantize \
        --width 512 \
        --height 512 \
        --steps 4 \
        --model schnell 2>&1; then
        # Verify the file exists and is a valid PNG
        if [[ -f "$IMAGE_PATH" ]]; then
            FILE_SIZE=$(stat -f%z "$IMAGE_PATH" 2>/dev/null || stat -c%s "$IMAGE_PATH" 2>/dev/null)
            if [[ "$FILE_SIZE" -gt 1000 ]]; then
                pass "flux generates a valid image (${FILE_SIZE} bytes)"
            else
                fail "flux image generation" "output file too small (${FILE_SIZE} bytes)"
            fi

            # Check PNG magic bytes
            MAGIC=$(xxd -l 4 -p "$IMAGE_PATH")
            if [[ "$MAGIC" == "89504e47" ]]; then
                pass "output is a valid PNG file"
            else
                fail "PNG validation" "file does not have PNG magic bytes (got $MAGIC)"
            fi
        else
            fail "flux image generation" "output file not created"
        fi
    else
        fail "flux image generation" "flux command failed"
    fi

    echo ""
fi

# ---------- Phase 2: Task-runner API tests ----------

if [[ "$RUN_API" == "true" ]]; then
    echo "--- Phase 2: task-runner API ---"
    echo ""

    # Test 2.1: Health check
    HEALTH=$(curl -sf "$TASK_RUNNER_URL/health" 2>/dev/null || echo "")
    if echo "$HEALTH" | grep -q '"healthy"'; then
        pass "task-runner health check"
    else
        fail "task-runner health check" "service not healthy at $TASK_RUNNER_URL"
        echo "  Skipping remaining API tests (service unreachable)"
        echo ""
        echo "=== Summary ==="
        echo "  $PASSED passed, $FAILED failed"
        exit 1
    fi

    # Test 2.2: Submit an image_generation task
    echo "  Submitting image_generation task..."
    SUBMIT_RESPONSE=$(curl -sf -X POST "$TASK_RUNNER_URL/tasks" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "image_generation",
            "params": {
                "prompt": "a blue square on a white background",
                "width": 512,
                "height": 512,
                "steps": 4,
                "model": "schnell",
                "quantize": true
            }
        }' 2>/dev/null || echo "")

    if [[ -z "$SUBMIT_RESPONSE" ]]; then
        fail "submit image_generation task" "empty response from API"
    else
        TASK_ID=$(echo "$SUBMIT_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [[ -z "$TASK_ID" ]]; then
            # Try alternate JSON key formats
            TASK_ID=$(echo "$SUBMIT_RESPONSE" | grep -o '"task_id":"[^"]*"' | head -1 | cut -d'"' -f4)
        fi

        if [[ -n "$TASK_ID" ]]; then
            pass "submit image_generation task (id: $TASK_ID)"
        else
            fail "submit image_generation task" "no task ID in response: $SUBMIT_RESPONSE"
        fi
    fi

    # Test 2.3: Poll for task completion
    if [[ -n "${TASK_ID:-}" ]]; then
        echo "  Waiting for task to complete (timeout: 300s)..."
        TIMEOUT=300
        INTERVAL=5
        ELAPSED=0
        TASK_STATUS=""

        while [[ $ELAPSED -lt $TIMEOUT ]]; do
            STATUS_RESPONSE=$(curl -sf "$TASK_RUNNER_URL/tasks/$TASK_ID" 2>/dev/null || echo "")
            TASK_STATUS=$(echo "$STATUS_RESPONSE" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)

            if [[ "$TASK_STATUS" == "completed" || "$TASK_STATUS" == "complete" ]]; then
                pass "task completed (${ELAPSED}s)"
                break
            elif [[ "$TASK_STATUS" == "failed" || "$TASK_STATUS" == "error" ]]; then
                TASK_ERROR=$(echo "$STATUS_RESPONSE" | grep -o '"error":"[^"]*"' | head -1 | cut -d'"' -f4)
                fail "task completion" "task failed: ${TASK_ERROR:-unknown error}"
                break
            fi

            sleep "$INTERVAL"
            ELAPSED=$((ELAPSED + INTERVAL))
            echo "    status: ${TASK_STATUS:-unknown} (${ELAPSED}s elapsed)"
        done

        if [[ $ELAPSED -ge $TIMEOUT ]]; then
            fail "task completion" "timed out after ${TIMEOUT}s (last status: ${TASK_STATUS:-unknown})"
        fi

        # Test 2.4: Verify output exists in task result
        if [[ "$TASK_STATUS" == "completed" || "$TASK_STATUS" == "complete" ]]; then
            OUTPUT_PATH=$(echo "$STATUS_RESPONSE" | grep -o '"output_path":"[^"]*"' | head -1 | cut -d'"' -f4)
            if [[ -z "$OUTPUT_PATH" ]]; then
                OUTPUT_PATH=$(echo "$STATUS_RESPONSE" | grep -o '"path":"[^"]*"' | head -1 | cut -d'"' -f4)
            fi

            if [[ -n "$OUTPUT_PATH" ]]; then
                pass "task result contains output path: $OUTPUT_PATH"

                # Test 2.5: Verify the generated image file exists
                if [[ -f "$OUTPUT_PATH" ]]; then
                    FILE_SIZE=$(stat -f%z "$OUTPUT_PATH" 2>/dev/null || stat -c%s "$OUTPUT_PATH" 2>/dev/null)
                    pass "generated image exists (${FILE_SIZE} bytes)"
                else
                    fail "generated image" "file not found at $OUTPUT_PATH"
                fi
            else
                echo "    (no output_path in response, skipping file check)"
            fi

            # Test 2.6: Verify thumbnail generation
            if [[ -n "$OUTPUT_PATH" ]]; then
                # Check common thumbnail path patterns
                THUMB_DIR=$(dirname "$OUTPUT_PATH")/thumbnails
                THUMB_FILE="$THUMB_DIR/$(basename "$OUTPUT_PATH")"
                ALT_THUMB="${OUTPUT_PATH%.*}_thumb.${OUTPUT_PATH##*.}"

                if [[ -f "$THUMB_FILE" ]]; then
                    THUMB_SIZE=$(stat -f%z "$THUMB_FILE" 2>/dev/null || stat -c%s "$THUMB_FILE" 2>/dev/null)
                    pass "thumbnail generated at $THUMB_FILE (${THUMB_SIZE} bytes)"
                elif [[ -f "$ALT_THUMB" ]]; then
                    THUMB_SIZE=$(stat -f%z "$ALT_THUMB" 2>/dev/null || stat -c%s "$ALT_THUMB" 2>/dev/null)
                    pass "thumbnail generated at $ALT_THUMB (${THUMB_SIZE} bytes)"
                else
                    # Check via API if thumbnails are exposed there
                    THUMB_RESPONSE=$(curl -sf "$TASK_RUNNER_URL/tasks/$TASK_ID/thumbnail" 2>/dev/null || echo "")
                    if [[ -n "$THUMB_RESPONSE" ]]; then
                        pass "thumbnail available via API"
                    else
                        fail "thumbnail generation" "no thumbnail found (checked $THUMB_DIR/ and $ALT_THUMB)"
                    fi
                fi
            fi
        fi
    fi

    echo ""
fi

# ---------- Summary ----------

echo "=== Summary ==="
echo "  $PASSED passed, $FAILED failed"

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi
