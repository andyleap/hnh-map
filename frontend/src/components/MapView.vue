<template>
    <div>
        <div ref="map" class="map"></div>
        <div class="control-panel card">
            <div class="card-body">
                <div class="form-group">
                    <div class="form-check">
                        <input type="checkbox" class="form-check-input" id="check-grid-coords"
                               v-model="showGridCoordinates">
                        <label class="form-check-label" for="check-grid-coords">Show grid coordinates</label>
                    </div>
                    <button type="button" class="btn btn-secondary" style="margin-top: 10px;" v-on:click="zoomOut">Zoom
                        out
                    </button>
                </div>
                <div class="form-group">
                    <label>Jump to Quest Giver</label>
                    <model-select :options="questGivers" v-model="selectedMarker"
                                  placeholder="Select Quest Giver"></model-select>
                </div>
                <div class="form-group">
                    <label>Jump to Player</label>
                    <model-select :options="players" v-model="zz" placeholder="Select Player"></model-select>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
    import {ModelSelect} from 'vue-search-select'
    import {CustomGridLayer, HnHCRS, HnHMaxZoom, HnHMinZoom, TileSize} from "../utils/LeafletCustomTypes";
    import * as L from "leaflet";
    import {API_ENDPOINT} from "../main";
    import {Marker} from "../data/Marker";
    import {UniqueList} from "../data/UniqueList";
    import {Character} from "../data/Character";

    export default {
        name: "MapView",
        components: {
            ModelSelect
        },
        data: function () {
            return {
                showGridCoordinates: false,

                trackingCharacterId: -1,
                autoMode: false,
                polling: null,
                zz: false,
                markersCache: [],
                questGivers: [],
                players: [],
                selectedMarker: {value: false}
            }
        },
        watch: {
            showGridCoordinates(value) {
                this.layer.showGridCoordinates(value);
            },
            trackingCharacterId(value) {
                if (value !== -1) {
                    let character = this.characters.byId(value);
                    if (character) {
                        this.map.setView(character.marker.getLatLng(), HnHMaxZoom);
                        this.$router.push({path: `/character/${value}`});
                        this.autoMode = true;
                    } else {
                        this.map.setView([0, 0], HnHMinZoom);
                        this.$router.replace({path: `/grid/0/0/${HnHMinZoom}`});
                        this.trackingCharacterId = -1;
                    }
                }
            },
            selectedMarker(value) {
                if (value) {
                    this.map.setView(value.marker.getLatLng(), this.map.getZoom());
                }
            }
        },
        mounted() {
            this.$http.get(`${API_ENDPOINT}/v1/characters`).then(response => {
                this.setupMap(response.body);
            }, () => this.$emit("error"));
        },
        beforeDestroy: function () {
            clearInterval(this.intervalId)
        },
        methods: {
            setupMap(characters) {
                // Create map and layer
                this.map = L.map(this.$refs.map, {
                    // Map setup
                    minZoom: HnHMinZoom,
                    maxZoom: HnHMaxZoom,
                    crs: HnHCRS,

                    // Disable all visuals
                    attributionControl: false,
                    inertia: false,
                    zoomAnimation: false,
                    fadeAnimation: false,
                    markerZoomAnimation: false
                });

                // Update url on manual drag, zoom
                this.map.on("drag", () => {
                    let point = this.map.project(this.map.getCenter(), this.map.getZoom());
                    let coordinate = {x: ~~(point.x / TileSize), y: ~~(point.y / TileSize), z: this.map.getZoom()};
                    this.$router.replace({path: `/grid/${coordinate.x}/${coordinate.y}/${coordinate.z}`});
                    this.trackingCharacterId = -1;
                });
                this.map.on("zoom", () => {
                    if (this.autoMode) {
                        this.autoMode = false;
                    } else {
                        let point = this.map.project(this.map.getCenter(), this.map.getZoom());
                        let coordinate = {x: ~~(point.x / TileSize), y: ~~(point.y / TileSize), z: this.map.getZoom()};
                        this.$router.replace({path: `/grid/${coordinate.x}/${coordinate.y}/${coordinate.z}`});
                        this.trackingCharacterId = -1;
                    }
                });

                this.layer = new CustomGridLayer({tileSize: TileSize});
                this.layer.addTo(this.map);

                this.markers = new UniqueList();
                this.characters = new UniqueList();

                // Create markers
                this.updateCharacters(characters);

                // Check parameters
                if (this.$route.params.characterId) { // Navigate to character
                    this.trackingCharacterId = +this.$route.params.characterId;
                } else if (this.$route.params.gridX && this.$route.params.gridY && this.$route.params.zoom) { // Navigate to specific grid
                    let latLng = this.toLatLng(this.$route.params.gridX * 100, this.$route.params.gridY * 100);
                    this.map.setView(latLng, this.$route.params.zoom);
                } else { // Just show a map
                    this.map.setView([0, 0], HnHMinZoom);
                }

                this.intervalId = setInterval(() => {
                    this.$http.get(`${API_ENDPOINT}/v1/characters`).then(response => {
                        this.updateCharacters(response.body);
                    }, () => {
                        clearInterval(this.intervalId);
                        this.$emit("error")
                    });
                }, 2000);
                // Request markers
                this.$http.get(`${API_ENDPOINT}/v1/markers`).then(response => {
                    this.updateMarkers(response.body);
                }, () => {
                    this.$emit("error")
                });
            },
            updateMarkers(markersData) {
                this.markers.update(markersData.map(it => new Marker(it)),
                    (marker) => { // Add
                        marker.add(this.map);
                        marker.setClickCallback(() => {
                            this.map.setView(marker.marker.getLatLng(), HnHMaxZoom);
                        });
                    },
                    (marker) => { // Remove
                        marker.remove(this.map);
                    });
                this.markersCache.length = 0;
                this.markers.getElements().forEach(it => this.markersCache.push(it));

                this.questGivers.length = 0;
                this.markersCache.filter(it => it.type === "quest").forEach(it => this.questGivers.push(it));
            },
            updateCharacters(charactersData) {
                this.characters.update(charactersData.map(it => new Character(it)),
                    (character) => { // Add
                        character.add(this.map);
                        character.setClickCallback(() => { // Zoom to character on marker click
                            this.trackingCharacterId = character.id;
                        });
                    },
                    (character) => { // Remove
                        character.remove(this.map);
                    },
                    (character, updated) => { // Update
                        character.update(this.map, updated);
                        if (this.trackingCharacterId == updated.id) {
                            this.map.setView(character.marker.getLatLng(), HnHMaxZoom);
                        }
                    }
                );
                this.players.length = 0;
                this.characters.getElements().forEach(it => this.players.push(it));
            },
            toLatLng(x, y) {
                return this.map.unproject([x, y], HnHMaxZoom);
            },
            zoomOut() {
                this.trackingCharacterId = -1;
                this.map.setView([0, 0], HnHMinZoom);
            }
        }
    }
</script>

<style>
    .map {
        height: 100vh;
    }

    .leaflet-container {
        background: #000;
    }

    .map-tile {
        border-bottom: 1px solid #404040;
        border-right: 1px solid #404040;
        color: #404040;
        font-size: 12px;
    }

    .map-tile-text {
        position: absolute;
        left: 2px;
        top: 2px;
    }

    .control-panel {
        position: absolute;
        top: 10%;
        left: 10px;
        z-index: 502;
    }
</style>