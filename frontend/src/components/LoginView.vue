<template>
    <div class="d-flex justify-content-center align-items-center container" style="height: 80vh;">
        <div class="col-md-6 align-self-center">
            <div class="card">
                <div class="card-body">
                    <h5 class="card-title">Enter Login and Password</h5>
                    <div class="form-group">
                        <input type="text" class="form-control" id="login" placeholder="Enter login" v-model="login">
                    </div>
                    <div class="form-group">
                        <input type="password" class="form-control" id="password" placeholder="Password"
                               v-model="password">
                    </div>
                    <button v-on:click="performAuth" class="btn btn-primary">Submit</button>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
    export default {
        name: "LoginView",
        data: function () {
            return {
                login: "",
                password: ""
            }
        },
        methods: {
            performAuth() {
                if (this.login.length > 0 && this.password.length > 0) {
                    this.$emit("authenticated", btoa(this.login + ":" + this.password));
                }
            }
        }
    }
</script>

<style scoped>

</style>