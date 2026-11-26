# Privacy

This page describes what tally, the analytics tool, records about visitors. If you run a site
that uses tally, you can link to it from your own privacy policy. It describes the software as
published; the person running a tally server is responsible for how they run it.

## In short

- No cookies, and nothing stored in your browser.
- No IP addresses stored, not even shortened ones.
- No way to follow you across sites, or across days on the same site.
- No personal data, no fingerprinting, no advertising, nothing sold or shared.

## What the script sends

When a page loads, the tally script sends one small request with:

| Field | Example | Why |
|---|---|---|
| Page URL, without the `#hash` and with every query parameter removed except `utm_*` and `ref` | `https://example.com/blog/post?utm_source=newsletter` | Which pages are read, and which campaigns bring readers |
| Referrer: the page that linked here, if the browser shares it | `https://news.ycombinator.com/item?id=1` | Where visitors come from. Only a visit's first referrer is kept, links within the same site are ignored, and the query string is dropped |
| Screen width in pixels | `390` | Whether the device is a phone, tablet or desktop |

While the page is visible, it counts how long it stays visible and sends that number when you
leave or switch tabs. It does not watch clicks, scrolling, typing or mouse movement.

Your browser also sends, as it does with every request, your IP address and its user agent
(browser and operating system name). The server uses them to compute the country and the visitor
ID described below, and does not write them anywhere. The IP address is also held in memory for up
to 10 minutes to limit how many requests one address can send.

## What is stored

For each visit, the server stores:

- the pages viewed, when, and how long each was visible;
- where the visit came from (the referrer's site and path, and any `utm_*` campaign values);
- the country, looked up from the IP address (the IP itself is not stored);
- the browser name, the operating system name, and the device type (Desktop, Tablet or Mobile);
- a visitor ID, explained next.

### The visitor ID

To count unique visitors without cookies, the server computes:

```
visitor ID = first 8 bytes of SHA-256( daily salt ‖ site ID ‖ IP address ‖ user agent )
```

- The **daily salt** is 32 random bytes, made new every day (UTC) and stored only on the tally
  server. A daily cleanup deletes every salt older than yesterday, after which nobody, including
  the person running the server, can recompute an ID or link it to an IP address.
- The **site ID** is part of the hash, so the same person gets unrelated IDs on different sites.
- Only the hash is stored. The IP address and the user agent string are not.

The result is that a visitor can be recognised within one day on one site (to count them once,
and to group page views into visits), and not beyond that.

## What is not collected

- Cookies, localStorage, sessionStorage or any other storage in your browser. The one exception
  is the site owner's own opt-out switch (`?tally=ignore`), which sets a flag in their own browser
  so their visits are not counted; it is never set for visitors.
- IP addresses, full user agent strings, screen resolution beyond its width, language, time zone,
  installed fonts, or any other fingerprinting signal.
- Anything you type, the page's content, or query parameters other than `utm_*` and `ref`.
- Visits by known bots and crawlers, and by automated browsers.

## How long it is kept

Visits and page views are deleted automatically after 24 months, or sooner if the site owner set
a shorter retention. Salts are deleted once they are older than yesterday.

## Where it is kept

On the tally server and its Postgres database, run by the site owner (typically on Heroku, in
the US or the EU region they chose). The data is not sent anywhere else. Country lookups use a
database file stored on the same server, so the IP address is not sent to a third party.

The hosting platform may keep its own request logs. Heroku's router, for example, logs each
request with the client's IP address; those logs are short-lived unless the site owner forwards
them to a log service.

## Do I need a cookie banner?

tally sets no cookies and stores nothing on the visitor's device, and it stores no personal data,
which is why sites that use it usually do not ask for consent for analytics. This is not legal
advice: whether your site needs a banner also depends on everything else it loads.

## Opting out

Visitors can block the script with any content blocker; the site keeps working. Site owners can
exclude their own visits by opening any tracked page once with `?tally=ignore` (undo it with
`?tally=track`).
