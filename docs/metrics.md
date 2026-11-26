# Metrics

How each number on the dashboard is counted. Everything is computed from two tables: one row per
**pageview** (an event) and one row per **visit** (a session). See [privacy.md](privacy.md) for
what is stored in them.

## Visits and visitors

A **visit** starts with a pageview and lasts as long as the same visitor keeps going: it ends
after 30 minutes without a pageview or a visible-time ping. A visit that runs past midnight stays
one visit. The visit's source, referrer, campaign, country, browser, OS and device are fixed when
it starts.

A **visitor** is a visitor ID (see [privacy.md](privacy.md#the-visitor-id)). IDs are derived from
a salt that changes every day, so the same person on two different days counts as two visitors.
Over a multi-day period, "unique visitors" therefore counts each person once per day they
visited, and is higher than the true number of people when people return. Plausible and other
cookie-free tools make the same trade-off; it is the price of not being able to follow anyone
across days.

## The six metrics

| Metric | Definition |
|---|---|
| Unique visitors | Distinct visitor IDs among the pageviews in the period |
| Visits | Visits that started in the period |
| Pageviews | Pageviews in the period, including repeated views of the same page |
| Views per visit | Average number of pageviews in those visits |
| Bounce rate | Share of those visits with exactly one pageview. Its change is shown in percentage points (pp), and a fall is good |
| Visit duration | Average engaged time of those visits |

**Engaged time** is the time a page was actually visible: the tracker counts while the tab is in
the foreground, pauses when it is hidden, and reports when the page is left or before an SPA
navigation. A tab left open in the background does not inflate it. A one-page visit still has an
engaged time, so bounces do not count as 0 seconds.

## Periods and comparison

| Period | Range (in the site's time zone) | Compared with | Chart |
|---|---|---|---|
| Today | Midnight until now | Yesterday, up to the same time | Hourly |
| Yesterday | The whole of yesterday | The day before | Hourly |
| Last 7 days | The last 7 days, including today | The 7 days before | Daily |
| Last 30 days | The last 30 days, including today | The 30 days before | Daily |
| This month | The 1st until now | Last month, up to the same elapsed time | Daily |
| Last month | The whole of last month | The month before | Daily |
| Last 12 months | This month and the 11 before | The 12 months before | Monthly |
| Custom | From and to dates, inclusive (up to 3 years) | The same number of days just before | Hourly up to 2 days, daily up to about 3 months, monthly beyond |

While a period is still running, the comparison is cut to the same elapsed time, so "today until
2 pm" is compared with "yesterday until 2 pm", not with all of yesterday. The chart shows the
previous period as a dashed line.

The change is `(current − previous) / previous`, in percent, and is left out when the previous
value is 0. For bounce rate it is the difference in percentage points, left out when either period
has no visits.

All days, hours and months are in the time zone set for the site, including across daylight
saving changes.

## Panels

Each panel lists the top 10 by unique visitors (ties broken by pageviews, then visits);
**Details** shows up to 300 with a search box.

### Sources

- **Source** is decided in this order: the `utm_source` URL parameter, then `ref`, then the
  referrer's site. Well-known sites and spellings get one name, so `google.de`, `www.google.com`,
  the Google app and `?utm_source=google` are all "Google"; `t.co`, `x.com` and `twitter.com` are
  all "X". Anything unknown is shown as its domain. A visit with none of these is **Direct**.
- **Referrer** is the referring page's domain and path, without its query string. Links from
  the site itself (including its subdomains) are not referrers.
- **UTM source, medium, campaign** are the `utm_*` values as written in the link.

### Pages

- **Top pages**: unique visitors and pageviews per path. **Time** is the average engaged time of
  that page's views; views with no engaged time reported (for example when the browser was closed
  abruptly) are left out of the average instead of counting as 0.
- **Entry pages**: the first page of each visit. **Exit pages**: the last.
- Paths are stored without the query string (except `utm_*` and `ref`, which become campaign
  values, not part of the path) and without the `#hash`.

### Countries

From the visitor's IP address, using the DB-IP "IP to Country Lite" database. The server
downloads the current month's edition each time it starts (Heroku restarts dynos daily, which keeps
it fresh). "Unknown" means the address is not in the database, or the database was not loaded yet
or could not be downloaded. Country data: [IP Geolocation by DB-IP](https://db-ip.com).

### Devices

Browser and OS come from the user agent. The device type comes from the screen width the tracker
reports: under 576 px is Mobile, under 992 px is Tablet, anything wider is Desktop. Without a
width, the user agent decides.

## Filters

Clicking any row filters the whole dashboard to it ("Source is Google"). Filters on different
dimensions combine with AND; a new filter on the same dimension replaces the old one.

- Filters on a visit's attributes (source, country, device, ...) select those visits and all
  their pageviews.
- A **page** filter selects the pageviews of that page, and the visits that included it. So with
  "Page is /pricing", pageviews counts views of /pricing only, while views per visit, bounce rate
  and visit duration describe the whole visits that included /pricing.
- **Entry page** and **exit page** filters select visits by their first or last page, and all of
  those visits' pageviews.

## Current visitors

Unique visitors with a pageview or engaged-time ping in the last 5 minutes. The dashboard
refreshes it every 30 seconds.

## What is not counted

- Known bots and crawlers, headless and automated browsers.
- Requests for an unknown site, or from a page whose hostname is not the site's domain or one
  of its subdomains.
- Pages on `localhost`, `127.0.0.1` or `*.local`, unless the site's domain is `localhost`.
- The site owner, after opening any tracked page once with `?tally=ignore`.
- Visitors whose content blocker blocks the script.
