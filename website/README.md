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

The workflow uploads `website/` directly from `main` and deploys it to GitHub Pages. No generated branch or branch pushes are needed. It uses the built-in GitHub Actions token with read-only repository access and Pages deployment permissions.

Keep **Settings → Pages → Source** set to **GitHub Actions**, and allow `main` in the `github-pages` environment's deployment branch rules. `website/CNAME` contains `www.foreverdubbed.com`; the custom domain must also be configured in repository settings.

Website-only pushes and pull requests skip `build.yml`. Changes to `.github/workflows/website.yml` are also excluded from app builds. Mixed app/website changes still build the app. Version tags and manual app builds still run, since GitHub does not apply path filters to tag pushes.

Website artwork is copied into `assets/` so website deployments are self-contained. Update those copies when changing the site's branding.

## Comparison claims

The comparison uses “Other voiceover addons” for prerecorded quest/gossip pack players, not every speech addon. Checked September 26, 2026 against the [original player's documentation](https://www.curseforge.com/wow/addons/voiceover), its [release features](https://github.com/mrthinger/wow-voiceover/releases), and a [WoW Forever pack-based implementation](https://www.curseforge.com/wow/addons/forever-voiceover). These document separate audio packs, quest/gossip playback, and queue controls; the Forever implementation explicitly describes missing lines needing pack coverage. Fixed recordings cannot synthesize arbitrary player names or changed text at playback time.

Forever Dubbed's claims follow the root README and `addon/ForeverDubbed/ForeverDubbed.lua`: dialogue text is sent to the local companion for speech. Its ambient speech handlers include NPC says, yells, and whispers; the [comparison player's event handlers](https://github.com/mrthinger/wow-voiceover/blob/master/AI_VoiceOver/VoiceOver.lua) cover quest and gossip interactions, not ambient NPC chat. Keep the text-availability qualification and desktop-companion tradeoff when updating this table. Recheck other addons before changing comparative claims.

## Downloads and screenshots

`site.js` recommends the Windows installer or macOS DMG using browser platform information. Both downloads remain accessible without JavaScript. Mobile visitors are not assigned a desktop download; the Mac requirement explicitly states Apple Silicon because browser OS detection cannot reliably determine the processor.

`assets/settings.png` and `assets/voices.png` are captures of the actual Fyne widgets rendered by the desktop test driver using example connection state, not captures of a running WoW session. Replace or supplement these with live screenshots as needed. An in-game screenshot showing a quest and the data square would be useful for the how-it-works section.

The Caudex fonts match the app and are served locally; their license is included in `assets/fonts/LICENSE.txt`. No external fonts, analytics, or JavaScript libraries are loaded by the site.
