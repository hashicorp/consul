import Route from './index';

export default class FolderRoute extends Route {
  templateName = 'dc/kv/index';

  beforeModel(transition) {
    super.beforeModel(...arguments);
    const params = this.paramsFor('dc.kv.folder');
    if (params.key === '/' || params.key == null) {
      return this.transitionTo('dc.kv.index');
    }
  }
}
