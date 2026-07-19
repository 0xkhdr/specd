<!-- specd:managed:steering/structure.md:v1 begin -->
# Steering: Structure

> Fill this in for your project. Map the code so a task can find its files without
> scanning the whole tree. Replace the prompts below.

## Layout
- **<dir>/** — <what lives here>
- **<dir>/** — <what lives here>

## Naming & patterns
- <naming convention a change must follow>
- <where tests live relative to source>

## Spec authoring format
- `design.md` decision contract: declare `references:` (the `R<n>` requirements it
  traces to), plus `boundaries:`, `interfaces:`, `invariants:`, `failure:`,
  `integration:`, `alternatives:`, `disposition:`, and `owner:`. An unknown
  reference is always refused; the full contract is required under the production
  profile.
- `tasks.md` optional trace/risk columns: `refs`, `kind`, `risk`, `complexity`,
  `capabilities`, `context`, `evidence`, `checks`. The six required columns alone are a valid table; the rest may be omitted —
  the production planning profile requires the full set.
<!-- specd:managed:steering/structure.md:v1 end -->
