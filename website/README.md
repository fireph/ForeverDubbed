# Forever Dubbed website

Static site for www.foreverdubbed.com. Edit `index.html`, `style.css`, `site.js`, and `assets/` here; no Node dependencies or build step are required.

Preview from the repository root:

```sh
python3 -m http.server 8000 --directory website
```

Open http://localhost:8000.

## Publishing

1. In repository **Settings → Pages**, select **GitHub Actions** as the publishing source.
2. Set **www.foreverdubbed.com** as the custom domain in those settings.
3. At your DNS provider, point the `www` CNAME record to `fireph.github.io`. Configure the apex domain following [GitHub's domain instructions](https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/managing-a-custom-domain-for-your-github-pages-site) to redirect foreverdubbed.com to www.foreverdubbed.com.
4. Enable **Enforce HTTPS** when the certificate is ready.
5. Push website changes to `main`, or run **Deploy website** manually from Actions.

The `pages` branch starts with an empty root commit. The workflow copies the contents of `website/` to the root of that branch, removes files deleted from the source, and commits only when the published files change. It never copies app source or app binaries and does not rewrite branch history. Edit the source on `main`, not the generated `pages` branch.

The same workflow deploys the website through GitHub Actions: pushes made with `GITHUB_TOKEN` do not trigger a separate Pages build. Keep **Settings → Pages → Source** set to **GitHub Actions**, even though the generated files also live on `pages`. No personal access token is needed. A CNAME file is not needed for Actions deployments; the custom domain is configured in repository settings.

Website-only pushes and pull requests skip `build.yml`. Changes to `.github/workflows/website.yml` are also excluded from app builds. Mixed app/website changes still build the app. Version tags and manual app builds still run, since GitHub does not apply path filters to tag pushes.

Website artwork is copied into `assets/` so website deployments are self-contained. Update those copies when changing the site's branding.

## Downloads and screenshots

`site.js` recommends the Windows installer or macOS DMG using browser platform information. Both downloads remain accessible without JavaScript. Mobile visitors are not assigned a desktop download; the Mac requirement explicitly states Apple Silicon because browser OS detection cannot reliably determine the processor.

`assets/settings.png` and `assets/voices.png` are captures of the actual Fyne widgets rendered by the desktop test driver using example connection state, not captures of a running WoW session. Replace or supplement these with live screenshots as needed. An in-game screenshot showing a quest and the data square would be useful for the how-it-works section.

The Caudex fonts match the app and are served locally; their license is included in `assets/fonts/LICENSE.txt`. No external fonts, analytics, or JavaScript libraries are loaded by the site.
