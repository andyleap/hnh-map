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
    }

    remove(map) {
        if (this.marker) {
            map.removeLayer(this.marker);
        }
    }

    add(map) {
        if(!this.hidden) {
            let icon = new ImageIcon({iconUrl: `${this.image}.png`});
            let position = map.unproject([this.position.x, this.position.y], HnHMaxZoom);
            this.marker = L.marker(position, {icon: icon, title: this.name});
            this.marker.addTo(map)
        }
    }

    jumpTo(map) {
        if (this.marker) {
            let position = map.unproject([this.position.x, this.position.y], HnHMaxZoom);
            this.marker.setLatLng(position);
        }
    }

    setClickCallback(callback) {
        if (this.marker) {
            this.marker.on("click", callback);
        }
    }

    setContextMenu(callback) {
        if(this.marker) {
            this.marker.on("contextmenu", callback);
        }
    }
}