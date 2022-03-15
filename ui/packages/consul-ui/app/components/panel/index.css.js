export default (css) => css`
  .panel {
    --padding-x: 14px;
    --padding-y: 14px;
  }
  .panel {
    position: relative;
  }
  .panel-separator {
    margin: 0;
  }

  .panel {
    --tone-border: var(--tone-gray-300);
    border: var(--decor-border-100);
    border-radius: var(--decor-radius-200);
    box-shadow: var(--decor-elevation-600);
  }
  .panel-separator {
    border: 0;
    border-top: var(--decor-border-100);
  }
  .panel {
    color: rgb(var(--tone-gray-900));
    background-color: rgb(var(--tone-gray-000));
  }
  .panel,
  .panel-separator {
    border-color: rgb(var(--tone-border));
  }
`;
