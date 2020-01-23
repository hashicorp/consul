behavior "regexp_issue_labeler" "panic_label" {
    regexp = "panic:"
    labels = ["crash"]
}

behavior "remove_labels_on_reply" "remove_stale" {
    labels = ["waiting-reply"]
}

poll "closed_issue_locker" "locker" {
  schedule             = "0 50 1 * * *"
  closed_for           = "720h" # 30 days
  max_issues           = 500
  sleep_between_issues = "5s"

  message = <<-EOF
    I'm going to lock this issue because it has been closed for _30 days_ ⏳. This helps our maintainers find and focus on the active issues.

    If you have found a problem that seems similar to this, please open a new issue and complete the issue template so we can capture all the details necessary to investigate further.
  EOF
}

poll "stale_issue_closer" "closer" {
    schedule = "0 22 23 * * *"
    no_reply_in_last = "480h" # 20 days
    max_issues = 500
    sleep_between_issues = "5s"
    labels = ["waiting-reply"]
    message = <<-EOF
    I'm going to close this issue due to inactivity (_30 days_ without response ⏳ ). This helps our maintainers find and focus on the active issues.

    If you feel this issue should be reopened, we encourage creating a new issue linking back to this one for added context. Thanks!
    EOF
}
