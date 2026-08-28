# Web research patterns

Qoder exposes a `WebSearch` tool (and `WebFetch` to read a promising result in
full). Use them as a verification supplement — not a replacement for the Verify
command, but an additional signal when local scripts alone cannot capture
quality.

---

## When to use WebSearch in the loop

| Goal type | Use WebSearch for | Example query |
|---|---|---|
| SEO content | Check competing pages, keyword signals | `[target keyword] filetype:md OR site:*.dev` |
| API correctness | Verify endpoint signatures, check for deprecations | `[library] [method] deprecated 2025 OR 2026` |
| Dependency versions | Confirm latest stable before updating | `[package name] latest stable version` |
| Best practices | Check if your approach matches current consensus | `[pattern] best practice [language] 2026` |
| Content accuracy | Ground-truth check generated facts | `[claim] site:official-source.com` |
| Bundle/perf baselines | Compare your score to current industry benchmarks | `[framework] bundle size benchmark 2026` |

---

## Pattern 1 — SEO content verification

Use when: optimising blog posts, landing pages, documentation for search.

After your local score script runs, supplement with:

```
WebSearch: [target keyword] to see what the top 3 results have in common.
Note: heading structure, content length, semantic coverage, internal links.
If top results consistently have trait X that your content lacks,
add "add trait X" as the next hypothesis.
```

This gives you signal that no local readability or keyword-density script can
provide — what the search engine is actually rewarding right now.

---

## Pattern 2 — API currency check

Use when: refactoring code that calls external libraries or APIs.

Before committing any API-surface change:

```
WebSearch: [library name] [method name] changelog 2026
WebSearch: [library name] [method name] deprecated
```

If search returns deprecation notices or breaking changes, note the current
replacement pattern and use that as the hypothesis instead.

This prevents iterating toward a working-but-deprecated solution that will
break on the next library update.

---

## Pattern 3 — Dependency version check

Use when: the Verify command suggests a dependency might be outdated, or when
optimising for security/bundle size.

```
WebSearch: [package name] npm latest 2026
WebSearch: [package name] security advisory
```

Cross-reference against what is in `package.json`, `go.mod`, `requirements.txt`
or equivalent. Use the delta as a hypothesis: "update [package] from X to Y,
check if metric improves."

---

## Pattern 4 — Best practice calibration

Use when: stuck after 5 consecutive discards and local ideas are exhausted.

```
WebSearch: [language/framework] [metric type] optimisation techniques 2026
WebSearch: how to improve [metric] in [stack]
```

Extract 3 concrete, actionable techniques from the top results — use `WebFetch`
on the most promising one if the snippet is too thin. Do not extract vague
advice. Add each as a separate iteration hypothesis. This restocks your
hypothesis pool with externally validated approaches.

---

## Pattern 5 — Benchmark calibration

Use when: you want to know if your current metric value is good relative to
the industry, not just relative to your own baseline.

```
WebSearch: [framework] [metric] benchmark 2026 average
```

If your metric is already at or above the industry median, note this and
shift the goal definition (e.g. from "reduce bundle size" to "reduce bundle
size while improving lighthouse score").

---

## Pattern 6 — Content accuracy check

Use when: the Verify command measures style/structure but not factual accuracy
(e.g. documentation, blog posts, runbooks).

```
WebSearch: [specific claim in content] site:[authoritative source]
```

If the authoritative source contradicts your content, flag this as a
required fix before the next iteration (accuracy issues override metric gains).

---

## Rules for using WebSearch

1. **Supplement, never replace.** The Verify command runs every iteration.
   Web research adds signal; it does not replace the metric.

2. **Search at the right time.** Patterns 1-3 supplement Phase 5 (Verify).
   Patterns 4-5 are for stuck recovery in Phase 1 (Review). Pattern 6
   runs in Phase 6 (Decide) when a kept iteration touches factual claims.

3. **Extract actionable hypotheses.** Never let a search result produce a
   vague conclusion ("content could be better"). Always turn the search
   result into a specific next hypothesis ("add a FAQ section with 3
   questions, which top-ranking competitors include").

4. **Log the research signal.** When a search result influences a hypothesis,
   note it in the results log description:
   `"added FAQ section (web research: top results for [kw] all include FAQ)"`

5. **Don't over-search.** Maximum one WebSearch call per iteration. If you are
   searching every iteration, your Verify command is probably too weak —
   strengthen the local script instead.

6. **Cite, don't guess.** `WebSearch` results come with source links; never
   turn an unverified snippet into a change that the Guard cannot catch.
