# gh-mysql-tools

General purpose MySQL tools by GitHub Engineering

### General notes

This repository publishes tools created and used by GitHub's Database Infrastructure team. These will be small tools/scripts/configs that we use internally at GitHub.

To be able to publish these tools, we strip out GitHub-specific code (e.g. integration with our chatops, monitoring etc.). We publish the tools "as is", under the MIT license, and without support. We do not expect to maintain the tools on this repository, though we may periodically update them based on internal development. We do not expect to support contributions, to regularly answer questions, to review pull requests.

You are free to use, fork etc. as per the license. You are encouraged to maintain your own version.

# The tools

### gh-mysql-rewind

Move MySQL back in time, decontaminate or un-split brain a MySQL server, restore it into replication chain.

[Read more](rewind/)
