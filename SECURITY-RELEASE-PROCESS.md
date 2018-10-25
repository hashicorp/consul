# Security Release Process

The CoreDNS project has adopted this security disclosures and response policy
to ensure responsible handling of critical issues.


## Product Security Team (PST)

Security vulnerabilities should be handled quickly and sometimes privately. 
The primary goal of this process is to reduce the total time users are vulnerable to publicly known exploits.

The Product Security Team (PST) is responsible for organizing the entire response including internal communication and external disclosure.

The initial Product Security Team will consist of the set of maintainers that volunteered.

### mailing lists

* security@coredns.io : for any security concerns. Received by Product Security Team members, and used by this Team to discuss security issues and fixes.
* coredns-distributors-announce@lists.cncf.io: for early private information on Security patch releases. see below how CoreDNS distributors can apply for this list.


## Disclosures

### Private Disclosure Processes

If you find a security vulnerability or any security related issues, 
please DO NOT file a public issue. Do not create a Github issue.
Instead, send your report privately to security@coredns.io.
Security reports are greatly appreciated and we will publicly thank you for it.

Please provide as much information as possible, so we can react quickly.
For instance, that could include:
- Description of the location and potential impact of the vulnerability;
- A detailed description of the steps required to reproduce the vulnerability (POC scripts, screenshots, and compressed packet captures are all helpful to us)
- Whatever else you think we might need to identify the source of this vulnerability

### Public Disclosure Processes

If you know of a publicly disclosed security vulnerability please IMMEDIATELY email security@coredns.io 
to inform the Product Security Team (PST) about the vulnerability so we start the patch, release, and communication process.

If possible the PST will ask the person making the public report if the issue can be handled via a private disclosure process
(for example if the full exploit details have not yet been published).
If the reporter denies the request for private disclosure, the PST will move swiftly with the fix and release process.
In extreme cases you can ask GitHub to delete the issue but this generally isn't necessary and is unlikely to make a public disclosure less damaging.

## Patch, Release, and Public Communication

For each vulnerability a member of the PST will volunteer to lead coordination with the "Fix Team"
and is responsible for sending disclosure emails to the rest of the community.
This lead will be referred to as the "Fix Lead."

The role of Fix Lead should rotate round-robin across the PST.

Note that given the current size of the CoreDNS community it is likely that the PST is the same as the "Fix team."
The PST may decide to bring in additional contributors for added expertise depending on the area of the code that contains the vulnerability.

All of the timelines below are suggestions and assume a Private Disclosure.
If the Team is dealing with a Public Disclosure all timelines become ASAP. 
If the fix relies on another upstream project's disclosure timeline, that will adjust the process as well.
We will work with the upstream project to fit their timeline and best protect our users.

### Fix Team Organization

These steps should be completed within the first 24 hours of disclosure.

- The Fix Lead will work quickly to identify relevant engineers from the affected projects and
  packages and CC those engineers into the disclosure thread. These selected developers are the Fix
  Team.
- The Fix Lead will get the Fix Team access to private security repos to develop the fix.


### Fix Development Process

These steps should be completed within the 1-7 days of Disclosure.

- The Fix Lead and the Fix Team will create a
  [CVSS](https://www.first.org/cvss/specification-document) using the [CVSS
  Calculator](https://www.first.org/cvss/calculator/3.0). The Fix Lead makes the final call on the
  calculated CVSS; it is better to move quickly than making the CVSS perfect.
- The Fix Team will notify the Fix Lead that work on the fix branch is complete once there are LGTMs
  on all commits in the private repo from one or more maintainers.

If the CVSS score is under 4.0 ([a low severity
score](https://www.first.org/cvss/specification-document#i5)) the Fix Team can decide to slow the
release process down in the face of holidays, developer bandwidth, etc. These decisions must be
discussed on the security@coredns.io mailing list.

### Fix Disclosure Process

With the Fix Development underway the CoreDNS Security Team needs to come up with an overall communication plan for the wider community. 
This Disclosure process should begin after the Team has developed a fix or mitigation 
so that a realistic timeline can be communicated to users.

**Disclosure of Forthcoming Fix to Users** (Completed within 1-7 days of Disclosure)

- The Fix Lead will create a github issue in CoreDNS project to inform users that a security vulnerability
has been disclosed and that a fix will be made available, with an estimation of the Release Date. 
It will include any mitigating steps users can take until a fix is available.

The communication to users should be actionable.
They should know when to block time to apply patches, understand exact mitigation steps, etc.

**Optional Fix Disclosure to Private Distributors List** (Completed within 1-14 days of Disclosure):

- The Fix Lead will make a determination with the help of the Fix Team if an issue is critical enough to require early disclosure to distributors.
Generally this Private Distributor Disclosure process should be reserved for remotely exploitable or privilege escalation issues. 
Otherwise, this process can be skipped.
- The Fix Lead will email the patches to coredns-distributors-announce@lists.cncf.io so distributors can prepare their own release to be available to users on the day of the issue's announcement.
Distributors should read about the [Private Distributor List](#private-distributor-list) to find out the requirements for being added to this list.
- **What if a distributor breaks embargo?** The PST will assess the damage and may make the call to release earlier or continue with the plan.
When in doubt push forward and go public ASAP.

**Fix Release Day** (Completed within 1-21 days of Disclosure)

- the Fix Team will selectively choose all needed commits from the Master branch in order to create a new release on top of the current last version released.
- Release process will be as usual.
- The Fix Lead will request a CVE from [DWF](https://github.com/distributedweaknessfiling/DWF-Documentation)
  and include the CVSS and release details.
- The Fix Lead will inform all users, devs and integrators, now that everything is public,
  announcing the new releases, the CVE number, and the relevant merged PRs to get wide distribution
  and user action. As much as possible this email should be actionable and include links on how to apply
  the fix to user's environments; this can include links to external distributor documentation.


## Private Distributor List

This list is intended to be used primarily to provide actionable information to
multiple distributor projects at once. This list is not intended for
individuals to find out about security issues.

### Embargo Policy

The information members receive on coredns-distributors-announce@lists.cncf.io must not be
made public, shared, nor even hinted at anywhere beyond the need-to-know within
your specific team except with the list's explicit approval. 
This holds true until the public disclosure date/time that was agreed upon by the list.
Members of the list and others may not use the information for anything other
than getting the issue fixed for your respective distribution's users.

Before any information from the list is shared with respective members of your
team required to fix said issue, they must agree to the same terms and only
find out information on a need-to-know basis.

In the unfortunate event you share the information beyond what is allowed by
this policy, you _must_ urgently inform the security@coredns.io
mailing list of exactly what information leaked and to whom. 

If you continue to leak information and break the policy outlined here, you
will be removed from the list.

### Contributing Back

This is a team effort. As a member of the list you must carry some water. This
could be in the form of the following:

**Technical**

- Review and/or test the proposed patches and point out potential issues with
  them (such as incomplete fixes for the originally reported issues, additional
  issues you might notice, and newly introduced bugs), and inform the list of the
  work done even if no issues were encountered.

**Administrative**

- Help draft emails to the public disclosure mailing list.
- Help with release notes.

### Membership Criteria

To be eligible for the coredns-distributors-announce@lists.cncf.io mailing list, your
distribution should:

1. Be an active distributor of CoreDNS component.
2. Have a user base not limited to your own organization.
3. Have a publicly verifiable track record up to present day of fixing security
   issues.
4. Not be a downstream or rebuild of another distributor.
5. Be a participant and active contributor in the community.
6. Accept the [Embargo Policy](#embargo-policy) that is outlined above.
7. Have someone already on the list vouch for the person requesting membership
   on behalf of your distribution.

### Requesting to Join

New membership requests are sent to security@coredns.io.

In the body of your request please specify how you qualify and fulfill each
criterion listed in [Membership Criteria](#membership-criteria).
