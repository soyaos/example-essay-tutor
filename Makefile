# example-essay-tutor — Compo (SoyaPack v0 Agent reference)
#
# The pack itself is prompts + templates + manifest; the only build-time
# tooling here is the end-to-end test suite under e2e/.

.PHONY: e2e e2e-verbose e2e-live vet

# Full E2E suite: manifest validation → OpenAI-compat gateway → kernel
# single-prompt fast path (mock upstream) → guide.v1 → HTML/PDF render
# (headless Chrome) → Chinese glyph (tofu) scan.
#
# Prerequisites:
#   - sibling checkout of the SoyaOS core monorepo at ../soyaos
#     (see the replace directive in e2e/go.mod)
#   - Chrome / Chromium (auto-detected; override with SOYAOS_CHROME=<path>)
e2e:
	cd e2e && go test ./...

e2e-verbose:
	cd e2e && go test ./... -v

# Opt-in live-upstream variant (burns real tokens — budget discipline:
# run once, manually). Requires SOYA_MODEL_API_KEY / SOYA_MODEL_BASE_URL /
# SOYA_MODEL_DEFAULT to point at a real OpenAI-compat backend. The parent-trial
# profile requires thinking to be explicitly disabled and writes outputs to
# the ignored trial-output/ directory.
e2e-live:
	cd e2e && COMPO_E2E_LIVE=1 SOYA_MODEL_ENABLE_THINKING=false COMPO_E2E_OUTPUT_DIR=../trial-output go test ./... -run TestE2E_FastPrompt_LiveUpstream -v

vet:
	cd e2e && go vet ./...
