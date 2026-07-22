(() => {
  "use strict";

  const elements = {
    title: document.querySelector("#title"),
    filename: document.querySelector("#filename"),
    orientation: document.querySelector("#orientation"),
    narration: document.querySelector("#narration"),
    recap: document.querySelector("#recap"),
    source: document.querySelector("#source"),
    sourcePane: document.querySelector("#source-pane"),
    sourceToggle: document.querySelector("#source-toggle"),
    play: document.querySelector("#play"),
    stop: document.querySelector("#stop"),
    previous: document.querySelector("#previous"),
    next: document.querySelector("#next"),
    voice: document.querySelector("#voice"),
    rate: document.querySelector("#rate"),
    rateValue: document.querySelector("#rate-value"),
    status: document.querySelector("#status"),
  };

  const state = {
    sentences: [],
    index: 0,
    playing: false,
    paused: false,
    utterance: null,
    voices: [],
    selectedVoice: null,
    selectedVoiceURI: null,
    playbackID: 0,
    activeNodes: [],
  };

  function element(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text) node.textContent = text;
    return node;
  }

  function list(items) {
    const ul = element("ul");
    for (const item of items || []) ul.append(element("li", "", item));
    return ul;
  }

  function renderOrientation(narration) {
    const heading = element("h2", "", "Why this matters");
    const why = element("p", "sentence", narration.why_it_matters);
    addSpeakable(why, `Why this page matters. ${narration.why_it_matters}`);
    elements.orientation.append(heading, why);
    if (narration.what_to_listen_for?.length) {
      const items = list(narration.what_to_listen_for);
      const listenHeading = element("h2", "", "What to listen for");
      addSpeakable(listenHeading, "Here is what to listen for.");
      [...items.children].forEach((item) => addSpeakable(item, item.textContent));
      elements.orientation.append(listenHeading, items);
    }
    if (narration.estimated_minutes) {
      const duration = element("p", "sentence", `About ${narration.estimated_minutes} minutes.`);
      addSpeakable(duration, `This briefing will take about ${narration.estimated_minutes} minutes.`);
      elements.orientation.append(duration);
    }
  }

  function addSpeakable(node, text, sourceIDs = []) {
    const index = state.sentences.length;
    node.classList.add("sentence");
    node.tabIndex = 0;
    node.addEventListener("click", () => playFrom(index));
    node.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") playFrom(index);
    });
    state.sentences.push({ text, node, sourceIDs });
  }

  function renderNarration(narration) {
    narration.sections.forEach((section) => {
      const article = element("article", "narration-section");
      article.id = `narration-${section.id}`;
      article.append(element("h2", "", section.heading));
      const paragraph = element("p", "narration-copy");
      section.sentences.forEach((sentence) => {
        const span = element("span", "sentence", sentence);
        addSpeakable(span, sentence, section.source_section_ids || []);
        paragraph.append(span);
      });
      article.append(paragraph);
      if (section.recall_question) {
        const recall = element("p", "recall sentence", `Pause and recall: ${section.recall_question}`);
        addSpeakable(recall, `Pause and recall. ${section.recall_question}`, section.source_section_ids || []);
        article.append(recall);
      }
      elements.narration.append(article);
    });
  }

  function renderRecap(narration) {
    const groups = [
      ["Remember", narration.remember],
      ["Decisions", narration.decisions],
      ["Actions", narration.actions],
      ["Verify in the source", narration.verify],
    ].filter(([, items]) => items?.length);
    if (!groups.length) return;
    elements.recap.append(element("h2", "", "At the end"));
    const grid = element("div", "recap-grid");
    groups.forEach(([title, items]) => {
      const card = element("section", "recap-card");
      const itemList = list(items);
      [...itemList.children].forEach((item) => {
        item.classList.add("sentence");
        addSpeakable(item, `${title}: ${item.textContent}`);
      });
      card.append(element("h3", "", title), itemList);
      grid.append(card);
    });
    elements.recap.append(grid);
  }

  function renderSource(sources) {
    for (const source of sources) {
      const section = element("section", "source-section");
      section.id = source.id;
      section.innerHTML = source.html;
      elements.source.append(section);
    }
  }

  function highlight(index) {
    state.activeNodes.forEach((node) => node.classList.remove("active"));
    state.activeNodes = [];
    const current = state.sentences[index];
    if (!current) return;
    current.node.classList.add("active");
    state.activeNodes.push(current.node);
    current.node.scrollIntoView({ behavior: "smooth", block: "center" });
    current.sourceIDs.forEach((id, sourceIndex) => {
      const source = document.getElementById(id);
      if (!source) return;
      source.classList.add("active");
      state.activeNodes.push(source);
      if (sourceIndex === 0) source.scrollIntoView({ behavior: "smooth", block: "center" });
    });
    elements.status.textContent = `Sentence ${index + 1} of ${state.sentences.length}`;
  }

  function selectedVoice() {
    return state.voices.find((voice) => voice.voiceURI === state.selectedVoiceURI) || state.selectedVoice;
  }

  function speakCurrent() {
    if (!state.sentences[state.index]) {
      stopPlayback();
      return;
    }
    speechSynthesis.cancel();
    const playbackID = ++state.playbackID;
    highlight(state.index);
    const utterance = new SpeechSynthesisUtterance(state.sentences[state.index].text);
    utterance.rate = Number(elements.rate.value);
    const voice = selectedVoice();
    if (voice) utterance.voice = voice;
    utterance.onend = () => {
      if (!state.playing || playbackID !== state.playbackID) return;
      state.index += 1;
      if (state.index >= state.sentences.length) {
        stopPlayback();
        elements.status.textContent = "Finished";
        return;
      }
      speakCurrent();
    };
    utterance.onerror = (event) => {
      if (playbackID !== state.playbackID) return;
      if (event.error === "interrupted" || event.error === "canceled") return;
      stopPlayback();
      elements.status.textContent = `Speech error: ${event.error}`;
    };
    state.utterance = utterance;
    state.playing = true;
    state.paused = false;
    elements.play.textContent = "Pause";
    speechSynthesis.speak(utterance);
  }

  function togglePlayback() {
    if (state.playing && !state.paused) {
      speechSynthesis.pause();
      state.paused = true;
      elements.play.textContent = "Resume";
      elements.status.textContent = "Paused";
      return;
    }
    if (state.playing && state.paused) {
      speechSynthesis.resume();
      state.paused = false;
      elements.play.textContent = "Pause";
      return;
    }
    speakCurrent();
  }

  function playFrom(index) {
    state.index = Math.max(0, Math.min(index, state.sentences.length - 1));
    state.playing = true;
    speakCurrent();
  }

  function stopPlayback() {
    state.playbackID += 1;
    speechSynthesis.cancel();
    state.playing = false;
    state.paused = false;
    state.utterance = null;
    elements.play.textContent = "Play";
  }

  function move(amount) {
    stopPlayback();
    state.index = Math.max(0, Math.min(state.index + amount, state.sentences.length - 1));
    highlight(state.index);
  }

  function loadVoices() {
    state.voices = speechSynthesis.getVoices().filter((voice) => voice.lang.startsWith("en"));
    state.voices.sort((a, b) => Number(b.localService) - Number(a.localService) || a.name.localeCompare(b.name));
    elements.voice.replaceChildren();
    state.voices.forEach((voice) => {
      const option = element("option", "", `${voice.name}${voice.localService ? " — local" : ""}`);
      option.value = voice.voiceURI;
      elements.voice.append(option);
    });
    const selected = state.voices.find((voice) => voice.voiceURI === state.selectedVoiceURI);
    if (selected) {
      state.selectedVoice = selected;
      elements.voice.value = selected.voiceURI;
      return;
    }
    if (!state.selectedVoiceURI) {
      const preferred = state.voices.find((voice) => voice.localService && /Samantha|Ava|Alex/i.test(voice.name));
      state.selectedVoice = preferred || state.voices[0] || null;
      state.selectedVoiceURI = state.selectedVoice?.voiceURI || null;
      if (state.selectedVoiceURI) elements.voice.value = state.selectedVoiceURI;
    }
  }

  function bindControls() {
    elements.play.addEventListener("click", togglePlayback);
    elements.stop.addEventListener("click", () => {
      stopPlayback();
      elements.status.textContent = "Stopped";
    });
    elements.previous.addEventListener("click", () => move(-1));
    elements.next.addEventListener("click", () => move(1));
    elements.voice.addEventListener("change", () => {
      state.selectedVoiceURI = elements.voice.value;
      state.selectedVoice = state.voices.find((voice) => voice.voiceURI === state.selectedVoiceURI) || null;
    });
    elements.rate.addEventListener("input", () => {
      elements.rateValue.textContent = `${Number(elements.rate.value).toFixed(2).replace(/0$/, "")}×`;
    });
    elements.sourceToggle.addEventListener("click", () => {
      const visible = elements.sourcePane.classList.toggle("visible");
      elements.sourceToggle.textContent = visible ? "Hide source" : "Show source";
    });
    window.addEventListener("beforeunload", () => speechSynthesis.cancel());
  }

  async function start() {
    if (!("speechSynthesis" in window)) {
      elements.status.textContent = "This browser does not support speech synthesis.";
      return;
    }
    bindControls();
    loadVoices();
    if (speechSynthesis.addEventListener) {
      speechSynthesis.addEventListener("voiceschanged", loadVoices);
    } else {
      speechSynthesis.onvoiceschanged = loadVoices;
    }

    try {
      const response = await fetch("data.json", { cache: "no-store" });
      if (!response.ok) throw new Error(`Reader data returned ${response.status}`);
      const readerDocument = await response.json();
      const narration = readerDocument.narration;
      document.title = `${narration.title} — Planreader`;
      elements.title.textContent = narration.title;
      elements.filename.textContent = readerDocument.file_name;
      state.sentences = [];
      renderOrientation(narration);
      renderNarration(narration);
      renderRecap(narration);
      renderSource(readerDocument.sources);
      highlight(0);
      elements.status.textContent = `${state.sentences.length} sentences ready`;
    } catch (error) {
      elements.title.textContent = "The reader could not load";
      elements.status.textContent = error.message;
    }
  }

  start();
})();
