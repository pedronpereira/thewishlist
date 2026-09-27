# The Wishlist: Technical TODOs

This document records unfinished or fragile areas found during a review of the
current codebase.

## Correctness

- [ ] Return constructed `echo.NewHTTPError` values from every handler.
  Several handlers currently create an HTTP error but continue executing.
- [ ] Save newly created wish items before returning `201 Created`.
  The create path currently adds a new item to memory and returns before calling
  the store.
- [ ] Handle a missing item in the purchase handler without continuing to save
  and render a `nil` item.
- [ ] Decide how `Wishlist.Count` should work and update it during mutations, or
  remove it if it is unnecessary.
- [x] Validate whether an item's type may change during an update. The current
  lookup only searches the collection selected by the new type.
  `Wishlist.UpdateItem` now looks up the item by id across all collections and
  moves it into the collection matching the new `ItemType` if it changed.

## Error handling and validation

- [ ] Make file-loading failures explicit instead of logging the error and
  returning an empty wishlist.
  - [x] Change `Store.Load` to return `(domain.Wishlist, error)`.
  - [x] Return contextual file-read and JSON-decoding errors from `FileStore`.
  - [x] Update the cache wrapper and application call sites for the new return
    signature.
  - [ ] Propagate the initial load failure out of `app.Init` instead of logging
    it and continuing with an empty wishlist.
  - [ ] Return refresh and recovery load failures to the HTTP client.
- [ ] Validate cloud-storage environment variables during application startup.
- [ ] Return storage failures consistently to HTTP clients.
- [ ] Add stronger request validation for required item fields and accepted item
  types.
- [ ] Define behavior for malformed or incomplete full-wishlist replacement
  requests.

## Concurrency and state

- [ ] Protect the package-level in-memory wishlist from concurrent access, or
  remove the shared mutable copy and let the store own synchronization.
- [ ] Avoid lost updates when multiple visitors purchase or edit items at the
  same time.
- [ ] Review whether the unused `internal/cache` package should be completed or
  removed.

## Storage

- [ ] Consider atomic file replacement so an interrupted file write cannot
  corrupt the wishlist.
- [ ] Consider formatting saved JSON to keep changes readable in version
  control.

## Testing

- [ ] Add unit tests for `Wishlist.AddItem`, `IndexOf`, `UpdateItem`, and
  `ItemPurchased`.
- [ ] Add handler tests for successful requests, invalid payloads, missing
  items, and storage failures.
- [ ] Add storage tests for JSON loading and saving.
- [ ] Add integration tests for the HTMX purchase flow.
- [ ] Run tests and static analysis in CI.

## Build and deployment

- [ ] Fix or remove the CI step that copies the currently absent `resources`
  directory.
- [ ] Confirm that the reusable GitHub Actions workflow is called by a separate
  workflow; it currently declares only `workflow_call`.
- [ ] Pin or update GitHub Action versions as part of routine maintenance.
- [ ] Make runtime asset paths independent of the process working directory.
- [ ] Document all supported environment variables.
- [ ] Add container health checks and clarify persistent storage requirements
  for the JSON-file mode.
- [ ] Avoid treating data embedded in a container image as durable writable
  application storage.

## Front end and security

- [ ] Pin or self-host the HTMX and Idiomorph browser dependencies.
- [ ] Add integrity and security controls if CDN scripts remain in use.
- [ ] Decide whether mutation endpoints require authentication or other access
  control.
- [ ] Add CSRF protection if the application will accept mutations from
  untrusted browser sessions.
- [ ] Validate or constrain externally supplied image and shop URLs.
- [ ] Improve accessibility with meaningful image alternative text and clearer
  status feedback.

## Documentation and maintenance

- [ ] Expand the README with environment configuration, API examples, and an
  accurate Docker run command.
- [ ] Document the accepted `ItemType` values and fallback behavior.
- [ ] Decide whether backup JSON files belong in source control or in an
  external backup system.
- [ ] Remove committed build executables and temporary build output if they are
  not intentionally distributed through the repository.

## Tech debt (deferred)

Only work on these items when no higher-priority tasks remain.

### MongoDB

- [ ] Replace the hard-coded MongoDB document ID with configuration or a
  suitable query.
- [ ] Reuse and manage a MongoDB client instead of connecting and disconnecting
  for every load and save.
- [ ] Add MongoDB timeouts and cancellation-aware contexts.
- [ ] Verify database connectivity during startup.
- [ ] Return MongoDB disconnect failures instead of panicking in deferred
  cleanup.
- [ ] Use operation-specific context in MongoDB errors; query and decode
  failures should not be described as connection failures.
