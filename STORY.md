# The Ore Story

## October 2024: Why Books Fail at Teaching Language Transitions

It started with a book idea: _"Go for Rubyists"_. A guide for experienced Ruby developers wanting to learn Go. But halfway through writing, the fundamental problem became clear: **books are the wrong medium for teaching programming transitions**.

Here's why books fail:

You're not teaching basic programming logic. You're teaching experienced developers who already understand concepts like HTTP clients (Faraday), HTML parsing (Nokogiri), AST manipulation, and object attributes. But books force a linear narrative.

The reader might know everything in chapter 5 but miss a crucial detail in chapter 3 they skipped. Or you repeat context in every chapter and sound condescending. Or you assume knowledge and lose readers who took a different path through the content. There's no way to win.

Books require you to keep readers interested while maintaining flow, but experienced developers don't need entertainment—they need working code they can study.

**The solution: Kill the book. Build a living project instead.**

If you want to leave Ruby for Go, your farewell is reading the production software you already use. If you want to master both, you learn by studying a tool you'll use daily. Code as documentation. No chapters, no forced narrative, no skipping crucial details.

UV had just exploded in the Python ecosystem (February 2024), proving that a Rust-based package manager could be 10-100x faster than the native tools. The Ruby community watched with envy. Why couldn't Ruby have something like that?

That question became the vehicle: build a fast Ruby package manager in Go. Not as a book example, but as real production software that teaches by existing.

That's how Ore was born.

## Why Go, Not Rust?

The obvious question: if UV used Rust and proved it could work, why not follow that path?

The honest answer: **I was still learning Rust.**

With Go, I had friends who could help. With Rust, not so much. I also had baggage from the pre-Rust 1.0 community days—experiences that left me thinking the Rust community was toxic. (Turns out that's not true anymore, but impressions stick.)

There's also credibility. For Ruby developers, Go has legitimacy. Mike Perham (@mperham) built Sidekiq in Ruby and Faktory in Go. That blueprint matters. When Ruby developers see Go, they think "serious backend work." When they see Rust, they think "systems programming" or "infrastructure tools built by people who don't use Ruby."

And pragmatically? Go had the **Charm suite** (Lipgloss, Bubbletea). No need to play with raw terminal manipulation. Like I told someone on Discord when showing a sneak peek of the tree feature: I'm savage, but not stupid. I'm not reimplementing something battle-tested like Lipgloss and Bubble.

So I chose Go. PubGrub was already there, but I had to build the rest:
- RubyGems client libraries
- Gemfile parsers
- Extension builders

The Ruby-Go bridge wasn't ready. I built it. But at least I had friends to call when I got stuck.

## Why Not Just Contribute to Bundler?

This is the question everyone asks. Why build something new instead of improving what exists?

The truth is: it's better to build, prove, then show.

Contributing to an established project means navigating existing architectures, legacy decisions, and community consensus. It means convincing maintainers that your approach is worth the disruption. It means years of discussion before seeing results.

Building Ore independently meant freedom to experiment, to prove concepts, to move fast. Once it works—once it demonstrates clear value—then the conversation changes. You're not asking for permission to try something. You're showing results and asking "want to integrate this?"

Bundler is the Swiss Army knife of Ruby dependency management—it handles everything from resolution to download to install to runtime graph management. It's deeply integrated into the Ruby runtime experience. Ore isn't trying to replace that. Ore is the metal armor that augments specific parts: the download, install, and cache operations that are painfully slow.

Think of it as a performance layer. Bundler still does what Bundler does best. Ore just makes the slow parts fast.

## From ore_reference to Ore Light

The original Ore repository (`ore_reference`) became an experimental playground—full features, alternative providers, advanced orchestration. Every idea went in there.

But for adoption, complexity is the enemy. New tools need to feel familiar. They need to solve obvious problems without asking users to learn new paradigms.

That's why Ore Light exists.

Ore Light extracts the essential features from `ore_reference`:
- Fast parallel gem downloads (using Go's concurrency)
- Smart caching (checking system RubyGems cache before downloading)
- Native extension building (supporting C, Rust, CMake)
- Platform-specific filtering (no more downloading Linux gems on macOS)
- Bundler-compatible security auditing
- Beautiful dependency tree visualization
- Complete command parity with Bundler (21 commands)

It's not trying to be everything. It's trying to be the thing that makes your daily workflow faster without requiring you to change anything else.

## The Philosophy: Do One Thing Correctly

Bundler is the Swiss Army knife. It downloads, resolves, installs, caches, and manages runtime gem loading. It's deeply embedded in Ruby's ecosystem.

Ore Light is the focused tool. It handles gem bundling—the download, install, and cache parts—and does it fast. It understands Bundler's ecosystem, respects its conventions, and augments its pain points.

You keep using `bundle exec` if you want. Ore Light just makes `bundle install` obsolete by doing it faster, without requiring Ruby to even be installed.

This isn't about replacing Bundler. It's about evolving the toolchain.

## Who Is This For?

Anyone who values their time.

- Developers tired of waiting 5 minutes for `bundle install`
- CI/CD pipelines that waste compute time on gem downloads
- Teams working across multiple Ruby versions and platforms
- Anyone who wants `gem install` speed with `bundle install` correctness

If you've ever thought "there has to be a faster way," Ore Light is your answer.

## What's Next?

The code is here. The documentation is the codebase itself.

If you want to learn Go from Ruby, read Ore's source. It's production-ready, solves real problems, and demonstrates patterns you'll actually use. No toy examples. No contrived tutorials. Just working software.

This is how you teach language transitions—by building tools people actually use.

---

_Ore: Metal armor for your Ruby workflow._
