# caddy-anubis

caddy-anubis is a plugin that loads anubis for requests in order to slow down AI and Scraper traffic from destroying infrastucture.

I consider this current implementation more of a Proof-of-Concept. I am not sure how stable or well it works. This is my first Caddy plugin. I do not currently recommend it for production usage.

If you have experience with Caddy plugins, or see obvious issues in my code, feel free to open PRs or reach out to me.

## Known Issues

- One major issue is the very first request after a Caddy start or restart, takes like 5 seconds till anubis kicks in. All subsequent requests, even after clearing cookies, are near instant.

Note: Anubis removed all node packages installed via npm and during the install process with version 1.16.0. To update to the latest you have to fork anubis and update the .gitignore and remove the exclude `node_modules`. Run `make build` and then commit the new files. I did all that here: https://github.com/bentemple/anubis/commit/eed913867937562e126fd4ab8164da31430e70e0

## Current usage

Just add an `anubis` to your caddyfile in the block you want the protection. currently I have not seen it work properly inside a `route` or `handler` block. But it works outside of those.

There is an optional `target` field you can set if you want to force the redirect elsewhere. It does a 302 redirect.

Example (also check the caddyfile in this repo, it is used for testing):

```caddy

:80 {
  anubis

  # or 

  anubis {
    target https://qwant.com
  }
}
```

## Credits

- [anubis](https://github.com/TecharoHQ/anubis) - the project that started all of this.
