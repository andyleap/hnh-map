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
            this.marker.remove();
            this.marker = null;
        }
    }

    add(mapview) {
        if(!this.hidden) {
            let icon;
            
            if(this.image == "gfx/terobjs/mm/custom") {
                icon = new ImageIcon({iconUrl: 'gfx/terobjs/mm/custom.png', iconSize: [21, 23], iconAnchor: [11, 21], popupAnchor: [1, 3], tooltipAnchor: [1, 3]})
            } else {
                icon = new ImageIcon({iconUrl: `${this.image}.png`, iconSize: [32, 32]});
            }
            
            let position = mapview.map.unproject([this.position.x, this.position.y], HnHMaxZoom);
            this.marker = L.marker(position, {icon: icon, title: this.name});
            this.marker.addTo(mapview.markerLayer);
            this.marker.on("click", this.callClickCallback.bind(this));
            this.marker.on("contextmenu", this.callContextCallback.bind(this));
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