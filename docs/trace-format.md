# Trace format (sketch)

One record per line, fixed field order, and a length field that must agree with the
payload. The reader rejects a mismatch rather than guessing.

## Open questions

- Do we version the line itself, or is the first field enough?
- What should a reader do with a record whose session id is unknown: skip, or fail?
- Is the format stable enough to document as an interface for other tools?
