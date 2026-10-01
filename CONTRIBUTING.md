# Contributing to schrodeck

Thanks for your interest. schrodeck is early software. Issues, bug reports and test results from different Macs and Stream Deck models are especially welcome, and they need no paperwork.

## Code contributions require a CLA

schrodeck is licensed under the [Mozilla Public License 2.0](LICENSE). The maintainer may also offer schrodeck, or a product built on it, under other terms in the future (for example a commercial edition). To keep that possible, **pull requests that contain code or documentation can only be merged after the contributor signs a Contributor License Agreement (CLA).** The CLA grants the maintainer the right to relicense your contribution. You keep your copyright, and your contribution remains available under the MPL-2.0 in this repository.

The CLA process will be set up before the first outside pull request is accepted. If you plan to contribute code, open an issue first so we can agree on the approach.

## Ground rules

- **Public repo: no personal identifiers.** Don't include hostnames, usernames, device serials, Stream Deck `Device.UUID` values, tokens, or real profile manifests in issues, commits, or fixtures. Redact them as `<user>`, `<host>`, `<deck>`.
- Every change starts with an issue and lands through a pull request with green CI.
- Every check needs a known-bad test case that makes it fail, as well as a known-good one.
- See [CLAUDE.md](CLAUDE.md) for the full rules and [docs/adr/](docs/adr/) for the design decisions.

Not affiliated with or endorsed by Elgato or Corsair.
