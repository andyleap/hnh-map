import {HnHMaxZoom, ImageIcon} from "../utils/LeafletCustomTypes";
import * as L from "leaflet";

function detectType(name) {
    if (name === "gfx/invobjs/small/bush" || name === "gfx/invobjs/small/bumling") return "quest";
    if (name === "custom") return "custom";
    return name.substring("gfx/terobjs/mm/".length);
}

export class Marker {
    constructor(markerData) {
        this.id = markerData.id;
        this.position = markerData.position;
        this.name = markerData.name;
        this.image = markerData.image;
        this.type = detectType(this.image);
        this.marker = false;
        this.text = this.name;
        this.value = this.id;
        this.hidden = markerData.hidden;
        this.map = markerData.map;
        this.onClick = null;
        this.onContext = null;
    }

    remove(mapview) {
        if (this.marker) {
            mapview.map.removeLayer(this.marker);
            this.marker = null;
        }
    }

    add(mapview) {
        if(!this.hidden && this.map == mapview.mapid) {
            let icon = new ImageIcon({iconUrl: `${this.image}.png`});
            let position = mapview.map.unproject([this.position.x, this.position.y], HnHMaxZoom);
            this.marker = L.marker(position, {icon: icon, title: this.name});
            this.marker.addTo(mapview.map);
            this.marker.on("click", this.callClickCallback);
            this.marker.on("contextmenu", this.callContextCallback);
        }
    }

    update(mapview, updated) {
        if(this.map != updated.map) {
            this.remove(mapview);
        }
        this.map = updated.map;
        this.position = updated.position;
        if (!this.marker && this.map == mapview.mapid) {
            this.add(mapview);
        }
        if(this.marker) {
            let position = mapview.map.unproject([updated.position.x, updated.position.y], HnHMaxZoom);
            this.marker.setLatLng(position);
        }
    }

    jumpTo(map) {
        if (this.marker) {
            let position = map.unproject([this.position.x, this.position.y], HnHMaxZoom);
            this.marker.setLatLng(position);
        }
    }

    setClickCallback(callback) {
        this.onClick = callback;
    }

    callClickCallback(e) {
        if(this.onClick != null) {
            this.onClick(e);
        }
    }
    setContextMenu(callback) {
        this.onContext = callback;
    }

    callContextCallback(e) {
        if(this.onContext != null) {
            this.onContext(e);
        }
    }
}