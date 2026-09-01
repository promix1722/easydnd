# Search and agent discovery

The public search surface is deliberately small: `https://easydnd.org/` is the
only canonical, indexable page. Character, group, account and game URLs belong
to signed-in people and are not search content. `/legal` remains public for
people and agents that need the licence notice, but is not in the sitemap.

The site publishes three discovery files:

- `https://easydnd.org/robots.txt` permits public crawlers, including AI search
  and training crawlers, and keeps the `/v1/` application API out of crawl.
- `https://easydnd.org/sitemap.xml` lists the canonical homepage.
- `https://easydnd.org/llms.txt` gives text-oriented agents a short, factual
  product summary. It is an emerging convention, not a registration mechanism
  or a guarantee that an answer engine will use the site.

## Before submitting

Deploy the SEO release, then check the public responses rather than the source
tree:

```sh
curl -I https://easydnd.org/
curl https://easydnd.org/robots.txt
curl https://easydnd.org/sitemap.xml
curl https://easydnd.org/llms.txt
curl -I https://easydnd.org/characters/example
```

The first four URLs must return `200`. The three text/XML files must contain
their own content rather than `index.html`. `/` must not carry `X-Robots-Tag`;
the character URL must carry `X-Robots-Tag: noindex, nofollow`. Also fetch `/`
with JavaScript disabled, or inspect the curl body, and confirm that the title,
description, H1 and three product capabilities are present.

## Google Search Console

1. Open [Google Search Console](https://search.google.com/search-console/) and
   create a **Domain property** for `easydnd.org`. Use the DNS TXT record Google
   supplies; this covers the apex and `www` redirect without putting a private
   verification token in the repository.
2. Submit `https://easydnd.org/sitemap.xml` in **Sitemaps**.
3. Inspect `https://easydnd.org/` with **URL Inspection**, run the live test,
   confirm that rendered content and the canonical are visible, then request
   indexing once.
4. Validate the structured data with the
   [Rich Results Test](https://search.google.com/test/rich-results). Fix syntax
   errors; a WebApplication is still valid even when it does not claim ratings
   or a paid/free offer.
5. Review Page indexing and Search performance after Google has crawled the
   site. Repeated requests do not guarantee or accelerate inclusion.

Google documents that sitemaps help discovery but do not guarantee indexing:
<https://developers.google.com/search/docs/crawling-indexing/sitemaps/build-sitemap>.
Its JavaScript guidance also recommends server/static rendering because not all
bots execute JavaScript:
<https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics>.

## Yandex Webmaster

1. Open [Yandex Webmaster](https://webmaster.yandex.com/), add exactly
   `https://easydnd.org`, and verify it with the supplied DNS record.
2. Add `https://easydnd.org/sitemap.xml` under **Indexing → Sitemap files** and
   run its validation.
3. Submit `https://easydnd.org/` once under **Indexing → Reindex pages**, then
   add it to important-page monitoring.
4. Review Diagnostics, crawl status and excluded pages. The canonical page is
   English, which Yandex can index; do not configure Russian `hreflang` or an
   invented `/ru` URL.

Official setup and sitemap instructions:
<https://yandex.com/support/webmaster/en/service/quick-start> and
<https://yandex.com/support/webmaster/en/indexing-options/sitemap>.

## Bing and answer engines

After Google verification, open
[Bing Webmaster Tools](https://www.bing.com/webmasters/) and import the Google
Search Console property, or verify the domain separately. Confirm that the
sitemap was imported and inspect the homepage. With one stable public URL,
IndexNow adds key management and deployment work without useful freshness;
reconsider it when the site publishes a changing public compendium or other
content pages. Bing's current setup guide is
<https://www.bing.com/webmasters/help/getting-started-checklist-66a806de>.

There is no separate form that guarantees inclusion in ChatGPT Search. OpenAI
asks publishers to allow `OAI-SearchBot`; the generic allow rule does so and
also permits `ChatGPT-User` and `GPTBot`. See
<https://help.openai.com/en/articles/12627856>. The same generic rule permits
Perplexity's search and user-triggered crawlers; its crawler documentation is
<https://docs.perplexity.ai/docs/resources/perplexity-crawlers>.

After discovery, monitor referrals from answer engines in whatever traffic
analytics is deployed. Do not infer successful indexing from a crawler visit,
and do not infer failure from a `site:easydnd.org` query alone; use each
webmaster tool's URL inspection and coverage reports.
