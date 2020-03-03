import '@babel/polyfill'

import Vue from 'vue'
import App from './App.vue'
import VueResource from "vue-resource"
import router from './router'
import {HnHMaxZoom} from "./utils/LeafletCustomTypes";

export const API_ENDPOINT = `api`;

export function getTileUrl(x, y, zoom) {
    return `grids/${HnHMaxZoom - zoom}/${x}_${y}.png`
}

Vue.config.productionTip = false;

Vue.use(VueResource);

new Vue({
    router,
    render: h => h(App)
}).$mount('#app');
