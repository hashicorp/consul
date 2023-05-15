/* eslint-disable no-console */

import Component from '@glimmer/component';
export default class Poapis extends Component {
  fetchData = () => {
    return fetch('https://jsonplaceholder.typicode.com/posts/1', {
      referrerPolicy: 'strict-origin-when-cross-origin',
      method: 'GET',
    });
  };

  fetchAuth = () => {
    fetch('https://fe.dev.protectonce.com/frontegg/identity/resources/auth/v1/user', {
      method: 'POST',
      body: JSON.stringify({
        email: 'aditya.j@protectonce.com',
        password: 'Sp@cebound18#d3v',
        recaptchaToken: '',
        invitationToken: '',
      }),
      headers: { 'Content-Type': 'application/json' },
    })
      .then((response) => response.json())
      .then((json) => {
        this.getApiRoutes(json?.accessToken);
        // return json?.accessToken;
      });
  };

  getPOAuthToken = () => {
    return this.fetchAuth()
      ?.then((res2) => {
        return res2;
      })
      ?.catch((err) => {
        console.log('err', err);
      });
    // ?.then((res) => {
    //   return res;
    // })
  };

  getApiRoutes = (token) => {
    fetch('https://gql.dev.protectonce.com/graphql', {
      body: '{"query":"query ($limit: Int, $nextToken: Int, $returnAllRoutes: Boolean, $query: String!, $application_id: String, $searchTerm: String) {\\n  getAPIRoutes(\\n    limit: $limit\\n    nextToken: $nextToken\\n    returnAllRoutes: $returnAllRoutes\\n    query: $query\\n    application_id: $application_id\\n    searchTerm: $searchTerm\\n  )\\n}\\n","variables":{"query":"{\\"bool\\":{\\"must\\":[{\\"match_all\\":{}},{\\"term\\":{\\"api.metadata.appId.keyword\\":\\"PO_2a31cf13-13df-410a-a430-928b0aeaee7c\\"}},{\\"term\\":{\\"api.metadata.workloadId.keyword\\":\\"a42f5d5e-fbbc-47df-bc94-c1c59aba6a43\\"}}]}}","searchTerm":"","returnAllRoutes":true,"nextToken":0,"limit":5000,"application_id":"PO_2a31cf13-13df-410a-a430-928b0aeaee7c"}}',
      method: 'POST',
      headers: {
        authorization: token,
        'content-type': 'application/json',
        getapitokenauth: token,
        'x-api-key': 'da2-7na4grko3bbztfqncpho7x4gfu',
      },
    })
      .then((response) => response.json())
      .then((json) => {
        console.log('apis', json);
        return json;
      });

    // fetch('https://gql.dev.protectonce.com/graphql', {
    //   headers: {
    //     accept: '*/*',
    //     authorization: token,
    //     'content-type': 'application/json',
    //     getapitokenauth: token,
    //   },
    //   referrer: 'https://dev.protectonce.com/',
    //   referrerPolicy: 'strict-origin-when-cross-origin',

    //   mode: 'cors',
    //   credentials: 'include',
    // });
  };

  constructor() {
    super(...arguments);
    this.getPOAuthToken();
    // console.log(apis);
  }
}
