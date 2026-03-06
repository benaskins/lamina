# Will Larson's Books in the Agentic Era

I dispatched seven AI agents across seventeen GitHub issues last Friday. They triaged stale issues, committed bug fixes, migrated items to the correct repos, and left detailed comments categorising what they could fix versus what needed my input. The whole thing took about three minutes.

This is not a flex. It's a data point about where software engineering is heading — and it made me think about which engineering leadership ideas survive the transition.

## What I actually did

The agents handled implementation: reading code, checking what was stale, making safe fixes, writing commit messages. What I did was different. I decided which issues mattered. I set the boundary — "don't make significant design decisions without me." I'd spent the morning redesigning how image generation worked across four repositories, choosing to rip out ComfyUI in favour of FLUX.1 on Apple Silicon, and redrawing the line between a generic task runner and a domain-specific image pipeline. The agents couldn't have made those calls. They don't know why the boundaries are where they are.

This is the job Will Larson writes about.

## What stays

*An Elegant Puzzle* and *Staff Engineer* are fundamentally about the judgment layer of engineering: setting technical direction, managing migrations, thinking in systems, knowing which problems to solve and which to leave alone.

That layer isn't going away. If anything, it's becoming the *whole* job for senior engineers. When implementation gets faster, the cost of building the wrong thing goes up proportionally. You can ship a bad architecture in an afternoon now. Taste becomes a load-bearing skill.

Larson's migration strategy framework — the idea that you finish migrations rather than starting new ones, that you sequence work to build momentum — maps directly onto agentic workflows. I can parallelise the execution, but I still need to decide the order, set the constraints, and know when to stop.

His writing on technical direction is even more relevant. When agents can generate plausible solutions to almost any well-scoped problem, the question shifts from "can we build it?" to "should we build it, and where does it belong?" That's architectural taste. That's the staff engineer's job.

## What fades

The parts of Larson's work rooted in human team dynamics translate less directly. Agents don't need career growth conversations, skip-levels, or carefully managed reorg transitions. The communication overhead frameworks — how to structure meetings, how to manage information flow across teams — assume human bandwidth constraints that don't apply when your collaborators are stateless processes you spin up and forget.

Team sizing models change too. Larson's guidance on team size (six to eight engineers) is grounded in coordination cost between humans. The coordination cost with agents is near zero — you just give them clear scope and let them run. The constraint becomes context quality, not headcount.

Sprint planning, velocity tracking, story points — these were always proxies for "how much can this group of humans accomplish in a fixed time?" When the answer is "as much as you can direct," the proxies lose their purpose.

## The shift

Here's what I think is actually happening: the skills Larson describes for staff engineers are becoming the baseline for *all* engineers who work with agents. You need systems thinking to set good boundaries. You need migration strategy to sequence work. You need technical judgment to know what's safe to delegate and what isn't.

The implementation skills don't disappear — you still need to read code, understand systems, spot when an agent has done something wrong. But they become table stakes rather than the main event.

Larson's books were written for the small number of engineers who'd moved beyond implementation into direction-setting. In the agentic era, that's everyone's job.
