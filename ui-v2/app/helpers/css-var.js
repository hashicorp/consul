import { helper } from '@ember/component/helper';
// TODO: Look at ember-inline-svg
const cssVars = {
  '--kubernetes-color-svg': `url('data:image/svg+xml;charset=UTF-8,<svg viewBox="0 0 21 20" xmlns="http://www.w3.org/2000/svg"><g fill-rule="nonzero" stroke="%23FFF" fill="none"><path d="M10.21 1.002a1.241 1.241 0 0 0-.472.12L3.29 4.201a1.225 1.225 0 0 0-.667.83l-1.591 6.922a1.215 1.215 0 0 0 .238 1.035l4.463 5.55c.234.29.59.46.964.46l7.159-.002c.375 0 .73-.168.964-.459l4.462-5.55c.234-.292.322-.673.238-1.036l-1.593-6.921a1.225 1.225 0 0 0-.667-.83l-6.45-3.08a1.242 1.242 0 0 0-.598-.12z" fill="%23326CE5"/><path d="M10.275 3.357c-.213 0-.386.192-.386.429v.11c.005.136.035.24.052.367.033.27.06.492.043.7a.421.421 0 0 1-.125.2l-.01.163a4.965 4.965 0 0 0-3.22 1.548 6.47 6.47 0 0 1-.138-.099c-.07.01-.139.03-.23-.022-.172-.117-.33-.277-.52-.47-.087-.093-.15-.181-.254-.27L5.4 5.944a.46.46 0 0 0-.269-.101.372.372 0 0 0-.307.136c-.133.167-.09.422.094.57l.006.003.08.065c.11.08.21.122.32.187.231.142.422.26.574.403.06.063.07.175.078.223l.123.11a4.995 4.995 0 0 0-.787 3.483l-.162.047c-.042.055-.103.141-.166.167-.198.063-.422.086-.692.114-.126.01-.236.004-.37.03-.03.005-.07.016-.103.023l-.003.001-.006.002c-.228.055-.374.264-.327.47.047.206.27.331.498.282h.006c.003-.001.005-.003.008-.003l.1-.022c.131-.036.227-.088.346-.133.255-.092.467-.168.673-.198.086-.007.177.053.222.078l.168-.029a5.023 5.023 0 0 0 2.226 2.78l-.07.168c.025.065.053.154.034.218-.075.195-.203.4-.35.628-.07.106-.142.188-.206.309l-.05.104c-.099.212-.026.456.165.548.191.092.43-.005.532-.218h.001v-.001c.015-.03.036-.07.048-.098.055-.126.073-.233.111-.354.102-.257.159-.526.3-.694.038-.046.1-.063.166-.08l.087-.159a4.987 4.987 0 0 0 3.562.01l.083.148c.066.021.138.032.197.12.105.179.177.391.265.648.038.121.057.229.112.354.012.029.033.069.048.099.102.213.341.311.533.219.19-.092.264-.337.164-.549l-.05-.104c-.064-.12-.136-.202-.207-.307-.146-.23-.267-.419-.342-.613-.032-.1.005-.163.03-.228-.015-.017-.047-.111-.065-.156a5.023 5.023 0 0 0 2.225-2.8l.165.03c.058-.039.112-.088.216-.08.206.03.418.106.673.198.12.045.215.098.347.133.028.008.068.015.1.022l.007.002.006.001c.229.05.45-.076.498-.282.047-.206-.1-.415-.327-.47l-.112-.027c-.134-.025-.243-.019-.37-.03-.27-.027-.494-.05-.692-.113-.081-.031-.139-.128-.167-.167l-.156-.046a4.997 4.997 0 0 0-.804-3.474l.137-.123c.006-.069.001-.142.073-.218.151-.143.343-.261.574-.404.11-.064.21-.106.32-.187.025-.018.06-.047.086-.068.185-.148.227-.403.094-.57-.133-.166-.39-.182-.575-.034-.027.02-.062.048-.086.068-.104.09-.168.178-.255.27-.19.194-.348.355-.52.471-.075.044-.185.029-.235.026l-.146.104A5.059 5.059 0 0 0 10.7 5.328a9.325 9.325 0 0 1-.009-.172c-.05-.048-.11-.09-.126-.193-.017-.208.011-.43.044-.7.018-.126.047-.23.053-.367l-.001-.11c0-.237-.173-.429-.386-.429zM9.79 6.351l-.114 2.025-.009.004a.34.34 0 0 1-.54.26l-.003.002-1.66-1.177A3.976 3.976 0 0 1 9.79 6.351zm.968 0a4.01 4.01 0 0 1 2.313 1.115l-1.65 1.17-.006-.003a.34.34 0 0 1-.54-.26h-.003L10.76 6.35zm-3.896 1.87l1.516 1.357-.002.008a.34.34 0 0 1-.134.585l-.001.006-1.944.561a3.975 3.975 0 0 1 .565-2.516zm6.813.001a4.025 4.025 0 0 1 .582 2.51l-1.954-.563-.001-.008a.34.34 0 0 1-.134-.585v-.004l1.507-1.35zm-3.712 1.46h.62l.387.483-.139.602-.557.268-.56-.269-.138-.602.387-.482zm1.99 1.652a.339.339 0 0 1 .08.005l.002-.004 2.01.34a3.98 3.98 0 0 1-1.609 2.022l-.78-1.885.002-.003a.34.34 0 0 1 .296-.475zm-3.375.008a.34.34 0 0 1 .308.474l.005.007-.772 1.866a3.997 3.997 0 0 1-1.604-2.007l1.993-.339.003.005a.345.345 0 0 1 .067-.006zm1.683.817a.338.338 0 0 1 .312.179h.008l.982 1.775a3.991 3.991 0 0 1-2.57-.002l.979-1.772h.001a.34.34 0 0 1 .288-.18z" stroke-width=".25" fill="%23FFF"/></g></svg>')`,
  '--terraform-color-svg': `url('data:image/svg+xml;charset=UTF-8,<svg viewBox="0 0 16 18" xmlns="http://www.w3.org/2000/svg"><g fill="none" fill-rule="evenodd"><path fill="%235C4EE5" d="M5.51 3.15l4.886 2.821v5.644L5.509 8.792z"/><path fill="%234040B2" d="M10.931 5.971v5.644l4.888-2.823V3.15z"/><path fill="%235C4EE5" d="M.086 0v5.642l4.887 2.823V2.82zM5.51 15.053l4.886 2.823v-5.644l-4.887-2.82z"/></g></svg>')`,
  '--nomad-color-svg': `url('data:image/svg+xml;charset=UTF-8,<svg viewBox="0 0 16 18" xmlns="http://www.w3.org/2000/svg"><g fill-rule="nonzero" fill="none"><path fill="%231F9967" d="M11.569 6.871v2.965l-2.064 1.192-1.443-.894v7.74l.04.002 7.78-4.47V4.48h-.145z"/><path fill="%2325BA81" d="M7.997 0L.24 4.481l5.233 3.074 1.06-.645 2.57 1.435v-2.98l2.465-1.481v2.987l4.314-2.391v-.011z"/><path fill="%2325BA81" d="M7.02 9.54v2.976l-2.347 1.488V8.05l.89-.548L.287 4.48.24 4.48v8.926l7.821 4.467v-7.74z"/></g></svg>')`,
  '--consul-color-svg': `url('data:image/svg+xml;charset=UTF-8,<svg viewBox="0 0 18 18" xmlns="http://www.w3.org/2000/svg"><g fill="none" fill-rule="evenodd"><path d="M8.693 10.707a1.862 1.862 0 1 1-.006-3.724 1.862 1.862 0 0 1 .006 3.724" fill="%23961D59"/><path d="M12.336 9.776a.853.853 0 1 1 0-1.707.853.853 0 0 1 0 1.707M15.639 10.556a.853.853 0 1 1 .017-.07c-.01.022-.01.044-.017.07M14.863 8.356a.855.855 0 0 1-.925-1.279.855.855 0 0 1 1.559.255c.024.11.027.222.009.333a.821.821 0 0 1-.642.691M17.977 10.467a.849.849 0 1 1-1.67-.296.849.849 0 0 1 .982-.692c.433.073.74.465.709.905a.221.221 0 0 0-.016.076M17.286 8.368a.853.853 0 1 1-.279-1.684.853.853 0 0 1 .279 1.684M16.651 13.371a.853.853 0 1 1-1.492-.828.853.853 0 0 1 1.492.828M16.325 5.631a.853.853 0 1 1-.84-1.485.853.853 0 0 1 .84 1.485" fill="%23D62783"/><path d="M8.842 17.534c-4.798 0-8.687-3.855-8.687-8.612C.155 4.166 4.045.31 8.842.31a8.645 8.645 0 0 1 5.279 1.77l-1.056 1.372a6.987 6.987 0 0 0-7.297-.709 6.872 6.872 0 0 0 0 12.356 6.987 6.987 0 0 0 7.297-.709l1.056 1.374a8.66 8.66 0 0 1-5.279 1.77z" fill="%23D62783" fill-rule="nonzero"/></g></svg>')`,
};
export function cssVar(params, hash) {
  return typeof cssVars[params[0]] !== 'undefined' ? cssVars[params[0]] : params[1];
}

export default helper(cssVar);
