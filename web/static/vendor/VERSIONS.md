# Vendored frontend libraries

Served from the embedded `web/static/vendor/` folder; no CDN is used at runtime.

| File | Library | Version | License | Source |
|---|---|---|---|---|
| `htmx.min.js` | htmx | 2.0.11 | 0BSD (`htmx.LICENSE`) | https://cdn.jsdelivr.net/npm/htmx.org@2.0.11/dist/htmx.min.js |
| `Sortable.min.js` | SortableJS | 1.15.7 | MIT (`Sortable.LICENSE`, header kept in file) | https://cdn.jsdelivr.net/npm/sortablejs@1.15.7/Sortable.min.js |
| `pico.min.css` | Pico CSS | 2.1.1 | MIT (`pico.LICENSE.md`, header kept in file) | https://cdn.jsdelivr.net/npm/@picocss/pico@2.1.1/css/pico.min.css |

htmx 2.x is still the npm `latest` tag (htmx 4 is on `next`); revisit when 4.x becomes `latest`.
To update, download the new files over these, update this table and check the board drag-and-drop,
inline editing and search still work.
