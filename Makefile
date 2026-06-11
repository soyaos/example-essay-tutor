# example-essay-tutor — Compo (SoyaPack v0 Agent reference)
#
# The pack itself is prompts + templates + manifest; the only build-time
# tooling here is the end-to-end test suite under e2e/.

.PHONY: e2e e2e-verbose e2e-live vet

# Full E2E suite: manifest validation → OpenAI-compat gateway → kernel
# 3-step prompt chain (mock upstream) → guide.v1 → HTML/PDF render
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
# SOYA_MODEL_DEFAULT to point at a real OpenAI-compat backend.
e2e-live:
	cd e2e && COMPO_E2E_LIVE=1 go test ./... -run TestE2E_PromptChain_LiveUpstream -v

vet:
	cd e2e && go vet ./...
