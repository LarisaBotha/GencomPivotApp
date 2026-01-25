class FormCard extends HTMLElement {
  constructor() {
    super();
  }

  async connectedCallback() {
    const filePath = '/components/formCard.html';

    try {
      const response = await fetch(filePath);
      const html = await response.text();
      
      // this.outerHTML = html.replace('<slot></slot>', this.innerHTML);
      this.outerHTML = html;
      
    } catch (error) {
      console.error('Error:', error);
    }
  }
}

customElements.define('form-card', FormCard);

class RegisterPivotModal extends HTMLElement {
  constructor() {
    super();
  }

  async connectedCallback() {
    const filePath = '/components/registerPivotModal.html';

    try {
      const response = await fetch(filePath);
      const html = await response.text();
      
      this.outerHTML = html;
      
    } catch (error) {
      console.error('Error:', error);
    }
  }
}

customElements.define('register-pivot-modal', RegisterPivotModal);

class StatusCard extends HTMLElement {
  constructor() {
    super();
  }

  async connectedCallback() {
    const filePath = '/components/statusCard.html';

    try {
      const response = await fetch(filePath);
      const html = await response.text();
      
      this.outerHTML = html;
      
    } catch (error) {
      console.error('Error:', error);
    }
  }
}

customElements.define('status-card', StatusCard);

class PieChart extends HTMLElement {
  constructor() {
    super();
  }

  async connectedCallback() {
    const filePath = '/components/pieChart.html';

    try {
      const response = await fetch(filePath);
      const html = await response.text();
      
      this.outerHTML = html;
      
    } catch (error) {
      console.error('Error:', error);
    }
  }
}

customElements.define('pie-chart', PieChart);