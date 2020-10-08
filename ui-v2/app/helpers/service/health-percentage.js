import { helper } from '@ember/component/helper';

export default helper(function serviceHealthPercentage([params] /*, hash*/) {
  const total = params.ChecksCritical + params.ChecksPassing + params.ChecksWarning;
  return {
    passing: Math.round((params.ChecksPassing / total) * 100),
    warning: Math.round((params.ChecksWarning / total) * 100),
    critical: Math.round((params.ChecksCritical / total) * 100),
  };
});
