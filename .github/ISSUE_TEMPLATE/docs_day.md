---
name: Consul docs day task
about: This is a HashiCorp internal documentation task for the purpose of the Consul docs day event.
labels: ['consul', 'placeholder']
assignees: 
    - karl-cardenas-coding
    - jkirschner-hashicorp
    - trujillo-adam
---

body:
  - type: markdown
    attributes:
      value: |
        Thanks for taking the time to fill out this Consul docs event task request!
 - type: dropdown
    id: Category
    attributes:
      label: Category
      description: What category is this task for?
      options:
        - Add/Improve Examples
        - Structural Improvements and Cleanup
        - Visual Aids
    validations:
      required: true
  - type: input
    id: Short Name
    attributes:
      label: Short Name
      description: Please provide a short name for this task. 
      placeholder: ex. PKI Overview
    validations:
      required: true
  - type: textarea
    id: Description
    attributes:
      label: Description
      description: Please provide details about the task?
      placeholder: Tell us what you see!
    validations:
      required: true

  - type: textarea
    id: link
    attributes:
      label: Documentation page link
      description: Please provide a link to the documentation page (if applicable)
      placeholder: ex: https://www.consul.io/docs/connect/gateways#gateways
    validations:
      required: true

  - type: textarea
    id: logs
    attributes:
      label: Relevant log output
      description: Please copy and paste any relevant log output. This will be automatically formatted into code, so no need for backticks.
      render: shell