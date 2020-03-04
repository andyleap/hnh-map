<template>
    <div id="app">
        <router-view v-on:error="login"/>
    </div>
</template>

<script>
    import 'bootstrap/dist/css/bootstrap.min.css'

    import Vue from 'vue'

    export default {
        name: "App",
        methods: {
            login: function() {
                window.location.href = '/';
            }
        }
    }
</script>

<style scoped>
</style>