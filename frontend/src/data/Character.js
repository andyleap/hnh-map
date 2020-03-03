import {HnHMaxZoom} from "../utils/LeafletCustomTypes";
import * as L from "leaflet";

export class Character {
    constructor(characterData) {
        this.name = characterData.name;
        this.position = characterData.position;
        this.type = characterData.type;
        this.id = characterData.id;
        this.marker = false;
        this.text = this.name;
        this.value = this.id;
    }

    getId() {
        return `${this.name}`;
    }

    remove(map) {
        if (this.marker) {
            map.removeLayer(this.marker);
        }
    }

    add(map) {
        let position = map.unproject([this.position.x, this.position.y], HnHMaxZoom);
        this.marker = L.marker(position, {title: this.name});
        this.marker.addTo(map)
    }

    update(map, updated) {
        if (this.marker) {
            let position = map.unproject([updated.position.x, updated.position.y], HnHMaxZoom);
            this.marker.setLatLng(position);
        }
    }

    setClickCallback(callback) {
        if (this.marker) {
            this.marker.on("click", callback);
        }
    }
}