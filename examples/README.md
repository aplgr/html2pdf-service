# HTML Examples

This directory contains sample HTML files demonstrating different use cases for the html2pdf-service. Each template uses [Bootstrap 5](https://getbootstrap.com/), Google Fonts (Inter & Merriweather), and [Bootstrap Icons](https://icons.getbootstrap.com/) via CDN with placeholder images from [picsum.photos](https://picsum.photos/).

## Files

- `invoice.html` – Invoice with company logo, line items, totals, and payment terms.
- `receipt.html` – Simple purchase receipt with transaction details.
- `certificate.html` – Certificate of completion with signature area and border.
- `event_ticket.html` – Event ticket featuring event info and QR/barcode placeholder.
- `resume.html` – Developer CV with sidebar for skills and contact info.
- `report.html` – Dashboard snapshot with KPIs and simple charts.
- `meeting_minutes.html` – Agenda, participants, decisions, and action items.
- `product_catalog.html` – Product list with images, descriptions, and prices.
- `newsletter.html` – Newsletter layout with feature story and call-to-action.
- `terms.html` – Terms & policy document with sections and last updated info.
- `web_article.html` – Blog-style article with author byline and related links.
- `batch_export.html` – Multiple short articles combined for batch export.
- `dynamic_form.html` – Interactive form using htmx to live-update a preview.
- `shipping_label.html` – Compact label with addresses, tracking number, and barcode.

## Convert an Example to PDF

Replace `&lt;your-token&gt;` with a valid API key.

```bash
curl -X POST http://localhost/api/v1/pdf \
  -H "X-API-Key: <your-token>" \
  -F "html=@examples/invoice.html" \
  --output invoice.pdf
```

### Fetch a Remote Page

The service can also fetch remote content directly:

```bash
curl -X GET "http://localhost/api/v1/pdf?url=https://example.com" \
  -H "X-API-Key: <your-token>" \
  --output page.pdf
```

Tokens must be supplied in all requests and standard rate limiting applies. Feel free to fork these templates and customize them for your own projects. All resources are loaded via public CDNs, and images use deterministic seeds from picsum.photos.
