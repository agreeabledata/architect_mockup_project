# Results

Regenerate with `go run ./scripts/compare -from results/`.

| model | seeds | mean final balance | gap to bot | finished | model calls | input tok | output tok | invalid actions | est. cost |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| bot | 1,2,3 | 1536.7 | +0 | 3/3 | 0 | 0 | 0 | 0 | $0.0000 |
| meta-llama/llama-3.3-70b-instruct | 1,2,3 | 1156.7 | -380 | 3/3 | 134 | 293502 | 14703 | 5 | $0.0341 |
| mistralai/mistral-small-3.2-24b-instruct | 1,2,3 | 1156.7 | -380 | 3/3 | 129 | 293332 | 22348 | 0 | $0.0332 |

## Analysis

meta-llama/llama-3.3-70b-instruct averaged 1156.7 (gap -380). It fought under 150 krones 4 times (seeds 1,2), refreshed the board 11 times versus the bot's 11 times on the same seeds, and recorded 5 invalid actions. mistralai/mistral-small-3.2-24b-instruct averaged 1156.7 (gap -380). It fought under 150 krones 3 times (seeds 1,2), refreshed the board 13 times versus the bot's 11 times on the same seeds, and recorded 0 invalid actions. Every scored run reached day 50.

## Failed runs

deepseek/deepseek-chat seed 1: no_action after 9 model calls (day 5, balance 270). The last response was prose about the next quest and contained no tool call.
google/gemini-2.5-flash-lite seed 1: no_action on the first call. The transcript stop_reason is error, with empty content and zero tokens.
