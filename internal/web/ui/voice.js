export const LOCAL_VOICE_MODEL_ID = "parakeet-tdt-0.6b-v3-int8";

export function voiceReady(voiceDTO) {
	if (!voiceDTO || voiceDTO.ready !== true) return false;
	const provider = voiceDTO.provider;
	const model = typeof voiceDTO.model === "string" ? voiceDTO.model.trim() : "";
	if (!model) return false;
	if (provider === "local") {
		return model === LOCAL_VOICE_MODEL_ID
			&& voiceDTO.model_status?.available === true
			&& voiceDTO.model_status?.installed === true;
	}
	return provider === "provider";
}

export function filterAvailableVoiceModels(models, search) {
	if (!Array.isArray(models)) return [];
	const query = String(search ?? "").trim().toLocaleLowerCase();
	return models.filter((model) => typeof model === "string"
		&& model.trim() !== ""
		&& (!query || model.toLocaleLowerCase().includes(query)));
}
