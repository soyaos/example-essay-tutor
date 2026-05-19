# example-essay-tutor — Compo (DD-008 flagship reference)

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
title their child has been assigned. Compo:

1. Reads the sample with `tool.parse_input` and infers 体裁 (genre), 年级
   (grade level), 文采特征 (stylistic features) and a few structural
   highlights.
2. Generates a `guide.v1` JSON document for the supplied title, covering an
   opening direction, three concrete writing points, eight 好词, five 好句,
   three 要避免的坑, and a model sample paragraph.
3. Refines the guide to the inferred grade so a third-grader's guide doesn't
   read like a tenth-grader's.
4. Renders the result to both HTML (web preview) and PDF (printable A4,
   complete with a 家长签名 line and a 辅导日期 row).

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
        "messages": [
          {"role":"user","content":"标题：难忘的一次劳动"}
        ]
      }'
```

The `soyaos` CLI is still under heavy construction; some of the above commands
will error until the corresponding milestones land (see the parent SoyaOS
roadmap for `soyaos agent build` / `deploy` status).

## Repository layout

```
example-essay-tutor/
├── soyapack.yaml          # Canonical v0 Agent manifest (entry point).
├── README.md              # You are here.
├── LICENSE                # MIT.
├── CHANGELOG.md           # Keep a Changelog v1.1.
├── CODE_OF_CONDUCT.md     # Contributor Covenant v2.1.
├── prompts/
│   ├── analyze_sample.md  # Stage 1 — sample → YAML report.
│   ├── generate_guide.md  # Stage 2 — YAML + title → guide.v1 JSON.
│   └── refine_for_grade.md# Stage 3 — JSON → grade-tuned JSON.
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

This is **v0.1.0-alpha.0** — the scaffold milestone. The prompts and templates
are deliberately complete enough to render a believable guide; the sample
inputs and expected outputs are placeholders until the first real
parent-tutoring round closes the loop.

| Milestone        | Status |
|------------------|--------|
| Manifest + scaffold (APP-465) | ✓ this release |
| Prompts + templates (APP-467) | ✓ this release |
| Real sample + expected output | — pending live parent run |
| `soyaos agent build` integration | — pending CLI milestone |
| Production deploy via Solo     | — pending |

## License

MIT — see [LICENSE](./LICENSE).
