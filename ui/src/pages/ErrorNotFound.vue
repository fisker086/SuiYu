<template>
  <q-layout>
    <q-page-container>
      <q-page class="flex flex-center column q-gutter-md">
        <div class="text-h3 text-weight-light">404</div>
        <div class="text-body1 text-text3">页面不存在</div>
        <q-btn color="primary" flat :to="{ path: '/' }">返回首页</q-btn>
      </q-page>
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { useErrorNotFound } from './errorNotFound'

defineOptions({
  name: 'ErrorNotFound'
})

useErrorNotFound()
</script>
