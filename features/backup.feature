# @manual — these scenarios require a live Proton account, the OS keyring, and an
# interactive browser for the first-run CAPTCHA. They cannot run headless in CI;
# a human runs them per docs/proton-backup-runbook.md and ticks each step.

Feature: Proton contacts + calendar backup CLI
  As a Proton user
  I want `./backup` to fetch and decrypt my contacts and calendar
  So that I have a git-versioned, client-readable backup tree

  @manual
  Scenario: First-run interactive login with CAPTCHA
    Given a clean machine with no proton-moresync keyring entries
    And the environment variable PROTON_USER set to my Proton address
    When I run "./backup --output-dir ~/proton-backup"
    Then I am prompted for my mailbox password
    And a browser opens at verify.proton.me for the CAPTCHA
    And after I paste the composite token the login succeeds
    And the session secrets are written to the OS keyring
    And the command prints "backup complete"

  @manual
  Scenario: Unattended re-run reuses the persisted session
    Given a previous successful run has populated the keyring
    When I run "./backup --output-dir ~/proton-backup </dev/null"
    Then no password or CAPTCHA prompt appears
    And the command prints "session reused"
    And the command prints "backup complete" with a zero exit code

  @manual
  Scenario: Backup tree contains standards-only files plus sidecars
    Given a successful backup run
    Then every "contacts/<uid>.vcf" file parses as RFC 6350 in a standard client
    And every "calendar/<cal-id>/<uid>.ics" file parses as RFC 5545
    And no .vcf or .ics file contains a Proton-specific "X-PM-" field
    And ".meta/contacts/<uid>.json" holds proton_id, cards, and version
    And ".meta/calendar/<cal-id>/<uid>.json" holds the key packets and parts

  @manual
  Scenario: Re-running is idempotent (no spurious commits)
    Given a successful backup run that was committed in the tree's git repo
    When I run the backup again with no account changes
    And the runner script proton-backup-sync.sh commits the tree
    Then "git status --porcelain" shows no changes
    And no new commit is created

  @manual
  Scenario: Locked session (9101) self-recovers
    Given the keyring holds a session whose scope has been locked by Proton
    When I run "./backup --output-dir ~/proton-backup"
    Then the tool prints "session locked — reconnecting with fresh login"
    And it re-derives keys and completes the backup
