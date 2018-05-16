Some helpful tips for a successful Apache Thrift PR:

* Did you test your changes locally or using CI in your fork?
* Is the Apache Jira THRIFT ticket identifier in the PR title?
* Is the Apache Jira THRIFT ticket identifier in the commit message?
* Did you squash your changes to a single commit?
* Are these changes backwards compatible? (please say so in PR description)
* Do you need to update the language-specific README?

Example ideal pull request title:

        THRIFT-9999: an example pull request title

Example ideal commit message:

        THRIFT-9999: [summary of fix, one line if possible]
        Client: [language(s) affected, comma separated, use lib/ directory names please]

For more information about committing, see CONTRIBUTING.md
