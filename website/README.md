Consul Website
==============

This subdirectory contains the entire source for the [Consul Website](https://www.consul.io). This is a [Middleman](http://middlemanapp.com) project, which builds a static site from these source files.

Contributions Welcome!
----------------------

If you find a typo or you feel like you can improve the HTML, CSS, or JavaScript, we welcome contributions. Feel free to open issues or pull requests like any normal GitHub project, and we'll merge it in.

Running the Site Locally
------------------------

Running the site locally is simple. Clone this repo and run `make dev`.

Then open up `localhost:4567`. Note that some URLs you may need to append ".html" to make them work (in the navigation and such).

Building Site
-------------

Building the static version of the site and running it is simple. Clone this repo and run the following commands:

```
$ bundle
$ bundle exec middleman build
$ foreman start
```

Then open up `localhost:5000`.

Alternately, the site can now be deployed to Heroku or Cloud Foundry.

