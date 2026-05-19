# Changelog

All notable changes to this SoyaPack will be documented in this file.

The format is based on [Keep a Changelog v1.1.0](https://keepachangelog.com/en/1.1.0/),
and this SoyaPack adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0-alpha.0] — 2026-05-19

### Added

- Initial SoyaPack v0 Agent scaffold for **Compo** — the DD-008 flagship
  reference Agent that turns a parent's essay-sample upload plus a writing
  title into a printable A4 PDF / HTML writing guide.
- `soyapack.yaml` manifest declaring `kind: Agent`, `virtual_model_id: soya:compo`,
  and both `html` + `pdf` artifacts under the `guide.v1` schema.
- 3-stage prompt chain under `prompts/`:
  - `analyze_sample.md` — read the range image / scan / text via
    `tool.parse_input`, emit a YAML report of 体裁 / 年级估算 / 文采特征 /
    结构亮点 / 学习要点.
  - `generate_guide.md` — fold the YAML report + the parent-provided title
    (+ optional `parent_hint`) into a `guide.v1` JSON document.
  - `refine_for_grade.md` — rewrite the JSON's vocabulary / good phrases /
    sample paragraph to fit the inferred grade level.
- Go `html/template` templates under `templates/`:
  - `guide.html.tmpl` — Soya "stone-ground warmth" visual system, three
    writing-point cards, two-column 好词 / 好句, 避坑提示, 范例段落.
  - `guide.pdf.tmpl` — A4 print variant with 家长签名 + 辅导日期 footer rows.
- Placeholder example inputs / outputs under `examples/`.
- Sandbox capability allowlist: `api.openai.com:443/https` egress only,
  `/workdir` read + `/workdir/out` write, `determinism_tier: read-only`.

[Unreleased]: https://github.com/soyaos/example-essay-tutor/compare/v0.1.0-alpha.0...HEAD
[0.1.0-alpha.0]: https://github.com/soyaos/example-essay-tutor/releases/tag/v0.1.0-alpha.0
