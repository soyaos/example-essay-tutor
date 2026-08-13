<p align="center">
  <img src="assets/logo.png" alt="SoyaOS" width="120" height="120" />
</p>

# example-essay-tutor — Compo (DD-008 flagship reference)

> [!WARNING]
> **This project is under active development and has not been formally
> released. APIs, manifests, prompts, and output schemas may introduce
> breaking changes at any time. Features are not yet stable; do not use this
> alpha as a production dependency.**

> *A parent uploads a photo of a good essay and a writing title. Thirty seconds
> later, a printable A4 writing guide comes out the other side.*

`example-essay-tutor` is the canonical SoyaPack v0 Agent reference for the
[DD-008 · Compo](https://github.com/soyaos/specs) flagship user story. It is
the smallest end-to-end example that exercises every Agent-shaped corner of
the SoyaPack spec — manifest, prompts, templates, artifacts, and the sandbox
capability allowlist — without bringing in any tooling the user wouldn't see
in production.

This repo is **a SoyaPack, not a Go program**. It is meant to be:

- Read top-to-bottom in ~5 minutes to learn the v0 Agent shape.
- Built into a `*.soyapack.tar.zst` archive by `soyaos agent build .`.
- Deployed into a SoyaOS Solo or kernel instance and invoked via the
  OpenAI-Compat virtual model id `soya:compo`.

## What Compo does

Compo is a writing tutor for elementary-school parents. The parent supplies
either a photograph / scan / paste of a good essay sample, plus the writing
title their child has been assigned. Compo's parent-trial profile:

1. Reads the title and sample with one fast prompt, infers a suitable grade,
   and returns bare, machine-parseable `guide.v1` JSON.
2. Enforces three writing points, eight 好词, five 好句, three 要避免的坑,
   non-empty fields, and a grade-appropriate model paragraph.
3. Renders the validated JSON to HTML (web preview) and PDF (printable A4,
   complete with a 家长签名 line and a 辅导日期 row).

The original analyze / generate / refine prompts remain in `prompts/` as
quality-mode reference material, but they are not on the 30-second trial path.

The whole flow is meant to fit inside a single 30-second sandbox budget — the
`budget_seconds_max: 30` line in `soyapack.yaml` is the contract.

## 30-second quickstart

```bash
# 1. Build the SoyaPack archive (writes dist/essay-tutor-0.1.0-alpha.0.soyapack.tar.zst).
soyaos agent build .

# 2. Validate the manifest in isolation.
soyaos agent validate ./soyapack.yaml

# 3. Deploy into a local Solo instance.
soyaos agent deploy ./dist/essay-tutor-0.1.0-alpha.0.soyapack.tar.zst

# 4. Invoke via the OpenAI-Compat gateway.
curl http://localhost:6473/v1/chat/completions \
  -H "Authorization: Bearer sk-soya-dev-local" \
  -H "Content-Type: application/json" \
  -d '{
        "model": "soya:compo",
        "response_format": {"type":"json_object"},
        "messages": [
          {"role":"user","content":"标题：难忘的一次劳动"}
        ]
      }'
```

The chat response is the validated JSON source for the HTML/PDF renderers. To
run the full lifecycle with a real model, enforce the 30-second limit, and
write all three artifacts into the ignored `trial-output/` directory:

```bash
cd ../soyaos
./scripts/verify-compo-e2e.sh
```

This command reads the core repo's private `.env`; it sets the optional
OpenAI-compatible `enable_thinking=false` vendor extension for the fast trial
profile. Never commit that `.env` or a provider key.

## Repository layout

```
example-essay-tutor/
├── soyapack.yaml          # Canonical v0 Agent manifest (entry point).
├── README.md              # You are here.
├── LICENSE                # MIT.
├── CHANGELOG.md           # Keep a Changelog v1.1.
├── CODE_OF_CONDUCT.md     # Contributor Covenant v2.1.
├── prompts/
│   ├── fast_guide.md      # Parent trial — one call → bare guide.v1 JSON.
│   ├── analyze_sample.md  # Quality-mode reference: sample analysis.
│   ├── generate_guide.md  # Quality-mode reference: guide generation.
│   └── refine_for_grade.md# Quality-mode reference: grade refinement.
├── templates/
│   ├── guide.html.tmpl    # Go html/template — web preview.
│   └── guide.pdf.tmpl     # Go html/template — A4 print variant.
└── examples/
    ├── README.md          # How to add new sample → expected pairs.
    ├── sample-input-1.txt # Placeholder Chinese essay paragraph.
    └── expected-guide-1.html
```

## Manifest highlights

```yaml
spec_version: soyapack.v0
kind: Agent
name: essay-tutor
virtual_model_id: soya:compo
artifacts:
  - { kind: html, schema: guide.v1 }
  - { kind: pdf,  schema: guide.v1 }
sandbox:
  isolation: container
  budget_seconds_max: 30
  capabilities:
    network_out:
      - { host: api.openai.com, port: 443, proto: https }
    fs_read:  [/workdir]
    fs_write: [/workdir/out]
    determinism_tier: read-only
```

The manifest is the contract. The validator at
[`pkg/soyapack.Validate`](https://github.com/soyaos/soyaos/tree/main/pkg/soyapack)
is the authoritative referee — anything it rejects, every SoyaOS runtime will
reject.

## Templates: Go `html/template`, not Nunjucks

Despite some early planning docs that mentioned `.njk` extensions, this repo
uses **`html/template` syntax** because that is what `pkg/artifact.HTMLRenderer`
consumes. The `HTMLRenderer` also auto-injects the `@media print` CSS block
mandated by DESIGN §9, so the templates here deliberately *do not* repeat
those rules — adding them would only cause two copies to fight at print time.

The visual system is the "Stone-Ground Warmth" Soya theme:

- Background: Soy Milk White (`#FBFAF5`)
- Text: Soy Sauce Black (`#2B2419`)
- Accent: Soybean Gold (`#E0A52C`)
- Headings: Plus Jakarta Sans (fallback PingFang SC for the Chinese glyphs)
- Pebble-soft radii: 14px controls, 20px cards, 28px featured panels.

See [`DESIGN.soya.md`](https://github.com/soyaos/specs) for the full system.

## Status

This is **v0.1.0-alpha.0**, an unstable pre-release. The technical parent-trial
path is executable end to end; real parent feedback is still required before
the workflow or `guide.v1` shape can be treated as stable.

| Milestone        | Status |
|------------------|--------|
| Manifest + scaffold (APP-465) | ✓ this release |
| Prompts + templates (APP-467) | ✓ this release |
| One-call `guide.v1` fast path | ✓ technical trial profile |
| HTML/PDF + ≤30s live gate | ✓ automated acceptance |
| Real parent feedback | — pending 3 sessions |
| Production release | — not released |

## License

MIT — see [LICENSE](./LICENSE).
