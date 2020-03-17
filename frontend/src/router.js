import Vue from 'vue'
import Router from 'vue-router'
import MapView from "./components/MapView";

Vue.use(Router);

export default new Router({
    routes: [
        {path: '/', component: MapView},
        {path: '/character/:characterId', component: MapView},
        {path: '/grid/:map/:gridX/:gridY/:zoom', component: MapView}
    ]
})
