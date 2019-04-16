# HashiCorp Learn

This project is a centralized home to HashiCorp's educational content providing a self-directed learning experience for practitioners at all skill levels.

Currently, this is a WIP, covering [Vault](https://vaultproject.io) to start, with content for additional products coming in the future.

## Architecture

This project uses Hashicorp's [base website template](https://github.com/hashicorp/middleman-template), built with [middleman](https://middlemanapp.com/),using [postcss](https://github.com/postcss/postcss), [babel](https://github.com/babel/babel), and [reshape](https://github.com/reshape/reshape).

This project uses a custom html parsing pipeline that enables it to use static-rendered [preact](https://preactjs.com/) components, that are optionally rehydrated on the client side.

See `/assets/js/components/readme.md` for information on working with components.

Request access to HashiCorp marketing team developer documentation for more detailed information on this architecture. Jeff Escalante can help you get access.

## Setup

- If your ruby version (`ruby -v`) is below `2.3.1`, make sure you have the latest version of ruby installed with `brew install ruby`
- Read the last line of `Gemfile.lock` (`BUNDLED WITH`) and install that exact version of `bundler`: `gem install bundler --version=1.16.1`
- Make sure [node.js](https://nodejs.org/en/) is installed
- Run `bash bootstrap.sh` script to get all the dependencies installed
- Run `bundle exec middleman` to compile the site

If you run into version errors with Bundler, try:

- Delete the local `vendor/bundle` directory and re-run `bootstrap.sh`
- Ensure that you have the correct version of Bundler: `bundler --version`

## Content

_Note:_ **Tracks** (e.g. 'Encryption as a Service') are a collection of **topics** (e.g. 'Transit secret re-wrapping') organized into a sequence.

Content for _Learn_ is stored as Markdown files with the following path structure: `source/<product>/<track>/<name-of-topic>.md`

Metadata that is _specific_ to that topic is stored within the frontmatter of the Markdown file.

```yml
name: 'Install Vault'
content_length: 2
description: 'Short description of the track contents'
id: install-vault
level: Beginner
products_used:
  - Vault
  - Nomad
---

```

Images to be used within topic content are stored in an `img` folder dedicated to that track. Example: `source/<product>/<track>/<img>/<name-of-asset>.jpg` These assets are then referenced in a Markdown file using standard syntax.

### Track Metadata

To organize topic content into tracks, `.yml` files stored in `data/<product>/<track-grouping>.yml` define metadata about the track (the name, icon used, etc.) as well as the topics and the order of the topics that make up that track.
