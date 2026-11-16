<template>
  <ChannelStatusV1View v-if="isActive" :metric="metric" />
  <ChannelStatusV2View v-else />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { getChannelMonitorMode, isChannelMonitorActiveMode } from '@/utils/featureFlags'
import ChannelStatusV1View from './ChannelStatusV1View.vue'
import ChannelStatusV2View from './ChannelStatusV2View.vue'

const isActive = computed(() => isChannelMonitorActiveMode())
const metric = computed(() => (getChannelMonitorMode() === 'custom' ? 'ttft' : 'latency'))
</script>
